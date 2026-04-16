package grantan

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Config struct {
	Port      string
	DataDir   string
	StaticDir string
}

type Server struct {
	mu       sync.RWMutex
	rooms    map[string]*room
	upgrader websocket.Upgrader
	config   Config
}

type clientConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func LoadConfig() Config {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "5678"
	}

	staticDir := strings.TrimSpace(os.Getenv("STATIC_DIR"))
	if staticDir == "" {
		staticDir = "public"
	}

	return Config{
		Port:      port,
		DataDir:   strings.TrimSpace(os.Getenv("DATA_DIR")),
		StaticDir: staticDir,
	}
}

func NewServer(config Config) *Server {
	return &Server{
		rooms: map[string]*room{},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		config: config,
	}
}

func (s *Server) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/games", s.handleListGames)
	mux.HandleFunc("POST /api/games", s.handleCreateGame)
	mux.HandleFunc("GET /api/games/{id}", s.handleGetGame)
	mux.HandleFunc("POST /api/games/{id}/join", s.handleJoinGame)
	mux.HandleFunc("POST /api/games/{id}/start", s.handleStartGame)
	mux.HandleFunc("POST /api/games/{id}/save", s.handleSaveGame)
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	mux.HandleFunc("/", s.handleStatic)

	server := &http.Server{
		Addr:              ":" + s.config.Port,
		Handler:           withLogging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Grantan listening on :%s", s.config.Port)
	err := server.ListenAndServe()
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListGames(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	rooms := make([]*room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.RUnlock()

	summaries := make([]RoomSummary, 0, len(rooms))
	for _, room := range rooms {
		room.mu.RLock()
		summaries = append(summaries, room.summaryLocked())
		room.mu.RUnlock()
	}

	slices.SortFunc(summaries, func(a, b RoomSummary) int {
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			return strings.Compare(a.ID, b.ID)
		}
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		return 1
	})

	writeJSON(w, http.StatusOK, map[string]any{"games": summaries})
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	room := s.getRoom(r.PathValue("id"))
	if room == nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	room.mu.RLock()
	state := room.stateLocked()
	room.mu.RUnlock()
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var request createGameRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	room := newRoom(request.PlayerName, request.GameName, request.AIPlayers)

	s.mu.Lock()
	for {
		if _, exists := s.rooms[room.id]; !exists {
			break
		}
		room.id = randomCode(4)
	}
	s.rooms[room.id] = room
	s.mu.Unlock()

	room.mu.RLock()
	state := room.stateLocked()
	hostID := room.hostID
	room.mu.RUnlock()

	writeJSON(w, http.StatusCreated, map[string]any{
		"gameId":   room.id,
		"playerId": hostID,
		"state":    state,
	})
}

func (s *Server) handleJoinGame(w http.ResponseWriter, r *http.Request) {
	room := s.getRoom(r.PathValue("id"))
	if room == nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	var request joinGameRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	room.mu.Lock()
	player, err := room.addHumanLocked(request.PlayerName)
	if err != nil {
		room.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state := room.stateLocked()
	clients := room.clientsLocked()
	room.mu.Unlock()

	broadcastState(clients, state)
	writeJSON(w, http.StatusCreated, map[string]any{
		"gameId":   room.id,
		"playerId": player.ID,
		"state":    state,
	})
}

func (s *Server) handleStartGame(w http.ResponseWriter, r *http.Request) {
	room := s.getRoom(r.PathValue("id"))
	if room == nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	var request actorRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	room.mu.Lock()
	err := room.startLocked(request.PlayerID)
	if err != nil {
		room.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state := room.stateLocked()
	clients := room.clientsLocked()
	room.mu.Unlock()

	broadcastState(clients, state)
	s.ensureAITurn(room.id)
	writeJSON(w, http.StatusOK, map[string]any{"state": state})
}

func (s *Server) handleSaveGame(w http.ResponseWriter, r *http.Request) {
	room := s.getRoom(r.PathValue("id"))
	if room == nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	var request actorRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	room.mu.Lock()
	if request.PlayerID != room.hostID {
		room.mu.Unlock()
		writeError(w, http.StatusForbidden, "only the host can save")
		return
	}
	filePath, err := room.saveLocked(s.config.DataDir)
	if err != nil {
		room.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	state := room.stateLocked()
	clients := room.clientsLocked()
	room.mu.Unlock()

	broadcastState(clients, state)
	writeJSON(w, http.StatusOK, map[string]string{"savedTo": filePath})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameID := strings.TrimSpace(r.URL.Query().Get("gameId"))
	playerID := strings.TrimSpace(r.URL.Query().Get("playerId"))
	if gameID == "" || playerID == "" {
		writeError(w, http.StatusBadRequest, "gameId and playerId are required")
		return
	}

	room := s.getRoom(gameID)
	if room == nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &clientConn{conn: conn}

	room.mu.Lock()
	if err := room.attachClientLocked(playerID, client); err != nil {
		room.mu.Unlock()
		_ = client.close()
		return
	}
	state := room.stateLocked()
	clients := room.clientsLocked()
	room.mu.Unlock()

	broadcastState(clients, state)

	defer func() {
		room.mu.Lock()
		room.detachClientLocked(playerID, client)
		state := room.stateLocked()
		clients := room.clientsLocked()
		room.mu.Unlock()

		broadcastState(clients, state)
		_ = client.close()
	}()

	for {
		var action wsAction
		if err := conn.ReadJSON(&action); err != nil {
			return
		}
		if err := s.handleWSAction(room, playerID, action); err != nil {
			_ = client.send(wsEnvelope{Type: "error", Error: err.Error()})
		}
	}
}

func (s *Server) handleWSAction(room *room, playerID string, action wsAction) error {
	room.mu.Lock()

	var err error
	switch action.Type {
	case "roll":
		err = room.rollLocked(playerID)
	case "build":
		err = room.buildLocked(playerID, action.Build)
	case "trade":
		err = room.tradeLocked(playerID, action.Give, action.Get)
	case "end_turn":
		err = room.endTurnLocked(playerID)
	case "save_game":
		if playerID != room.hostID {
			err = errors.New("only the host can save")
		} else {
			_, err = room.saveLocked(s.config.DataDir)
		}
	default:
		err = errors.New("unknown action")
	}

	if err != nil {
		room.mu.Unlock()
		return err
	}

	state := room.stateLocked()
	clients := room.clientsLocked()
	room.mu.Unlock()

	broadcastState(clients, state)
	s.ensureAITurn(room.id)
	return nil
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
		http.NotFound(w, r)
		return
	}

	cleanPath := path.Clean(r.URL.Path)
	if cleanPath == "." {
		cleanPath = "/"
	}

	if cleanPath == "/" {
		http.ServeFile(w, r, filepath.Join(s.config.StaticDir, "index.html"))
		return
	}

	candidate := filepath.Join(s.config.StaticDir, strings.TrimPrefix(cleanPath, "/"))
	if fileInfo, err := os.Stat(candidate); err == nil && !fileInfo.IsDir() {
		http.ServeFile(w, r, candidate)
		return
	}
	if fileInfo, err := os.Stat(candidate + ".html"); err == nil && !fileInfo.IsDir() {
		http.ServeFile(w, r, candidate+".html")
		return
	}
	http.NotFound(w, r)
}

func (s *Server) getRoom(id string) *room {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rooms[id]
}

func (c *clientConn) send(message wsEnvelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(message)
}

func (c *clientConn) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Close()
}

func broadcastState(clients []*clientConn, state RoomState) {
	payload := wsEnvelope{Type: "state", State: &state}
	for _, client := range clients {
		if client == nil {
			continue
		}
		_ = client.send(payload)
	}
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
