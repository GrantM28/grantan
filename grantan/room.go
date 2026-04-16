package grantan

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

var (
	errRoomStarted   = errors.New("game has already started")
	errRoomFinished  = errors.New("game is already finished")
	errNotHost       = errors.New("only the host can do that")
	errNotYourTurn   = errors.New("it is not your turn")
	errRollFirst     = errors.New("roll the dice first")
	errLobbyNotReady = errors.New("at least two total players are needed to start")
)

var buildCosts = map[string]map[Resource]int{
	"road": {
		ResourceBrick:  1,
		ResourceLumber: 1,
	},
	"settlement": {
		ResourceBrick:  1,
		ResourceLumber: 1,
		ResourceWool:   1,
		ResourceGrain:  1,
	},
	"city": {
		ResourceGrain: 2,
		ResourceOre:   3,
	},
}

type room struct {
	mu          sync.RWMutex
	id          string
	name        string
	hostID      string
	started     bool
	phase       string
	maxPlayers  int
	currentTurn int
	lastRoll    int
	turnNumber  int
	winnerID    string
	players     []*Player
	log         []LogEntry
	createdAt   time.Time
	updatedAt   time.Time
	clients     map[string]*clientConn
	aiRunning   bool
}

func newRoom(hostName, gameName string, aiPlayers int) *room {
	now := time.Now().UTC()
	host := newPlayer(hostName, false, 0)
	r := &room{
		id:         randomCode(4),
		name:       sanitizeGameName(gameName, host.Name),
		hostID:     host.ID,
		phase:      phaseLobby,
		maxPlayers: 4,
		players:    []*Player{host},
		createdAt:  now,
		updatedAt:  now,
		clients:    map[string]*clientConn{},
	}

	for i := 0; i < clampAIPlayers(aiPlayers); i++ {
		r.players = append(r.players, newPlayer(aiName(i), true, len(r.players)))
	}

	r.appendLogLocked(fmt.Sprintf("%s opened the lobby.", host.Name))
	for _, player := range r.players[1:] {
		r.appendLogLocked(fmt.Sprintf("%s joined as an AI player.", player.Name))
	}
	return r
}

func newPlayer(name string, isAI bool, order int) *Player {
	player := &Player{
		ID:        randomID(),
		Name:      sanitizePlayerName(name),
		IsAI:      isAI,
		Connected: true,
		Order:     order,
		Resources: map[Resource]int{},
	}
	for _, resource := range resourceOrder {
		player.Resources[resource] = 0
	}
	return player
}

func (r *room) addHumanLocked(name string) (*Player, error) {
	if r.started {
		return nil, errRoomStarted
	}
	if len(r.players) >= r.maxPlayers {
		return nil, errors.New("lobby is full")
	}

	player := newPlayer(name, false, len(r.players))
	r.players = append(r.players, player)
	r.updatedAt = time.Now().UTC()
	r.appendLogLocked(fmt.Sprintf("%s joined the lobby.", player.Name))
	return player, nil
}

func (r *room) getPlayerLocked(playerID string) *Player {
	for _, player := range r.players {
		if player.ID == playerID {
			return player
		}
	}
	return nil
}

func (r *room) currentPlayerLocked() *Player {
	if len(r.players) == 0 || r.currentTurn >= len(r.players) {
		return nil
	}
	return r.players[r.currentTurn]
}

func (r *room) startLocked(requesterID string) error {
	if r.started {
		return errRoomStarted
	}
	if requesterID != r.hostID {
		return errNotHost
	}
	if len(r.players) < 2 {
		return errLobbyNotReady
	}

	r.started = true
	r.phase = phaseRoll
	r.currentTurn = 0
	r.turnNumber = 1
	r.lastRoll = 0
	r.updatedAt = time.Now().UTC()

	// The game uses a simplified economy instead of a full board so it can stay
	// portable and fully in-memory. Each player starts with a small base so the
	// first few turns feel active right away.
	for _, player := range r.players {
		player.Roads = 0
		player.Settlements = 1
		player.Cities = 0
		player.VictoryPoints = 1
		for _, resource := range resourceOrder {
			player.Resources[resource] = 2
		}
	}

	r.appendLogLocked("The game has started.")
	r.appendLogLocked(fmt.Sprintf("It is %s's turn.", r.currentPlayerLocked().Name))
	return nil
}

func (r *room) rollLocked(playerID string) error {
	if !r.started {
		return errors.New("game has not started")
	}
	if r.phase == phaseFinished {
		return errRoomFinished
	}
	player := r.currentPlayerLocked()
	if player == nil || player.ID != playerID {
		return errNotYourTurn
	}
	if r.phase != phaseRoll {
		return errors.New("dice already rolled")
	}

	roll := rollDice()
	r.lastRoll = roll
	r.phase = phaseActions
	r.updatedAt = time.Now().UTC()

	// Instead of a hex board, structures generate random resources when a turn
	// begins. This keeps the state model small enough for a single-container,
	// database-free deployment while preserving the build/upgrade cadence.
	if roll == 7 {
		r.resolveSevenLocked(player)
	} else {
		for _, current := range r.players {
			r.produceResourcesLocked(current)
		}
	}

	r.appendLogLocked(fmt.Sprintf("%s rolled a %d.", player.Name, roll))
	return nil
}

func (r *room) resolveSevenLocked(active *Player) {
	for _, player := range r.players {
		if totalResources(player) > 7 {
			lost := removeRandomResource(player)
			if lost != "" {
				r.appendLogLocked(fmt.Sprintf("%s discarded 1 %s.", player.Name, lost))
			}
		}
	}

	var richest *Player
	for _, player := range r.players {
		if player.ID == active.ID || totalResources(player) == 0 {
			continue
		}
		if richest == nil || totalResources(player) > totalResources(richest) {
			richest = player
		}
	}
	if richest == nil {
		return
	}

	stolen := removeRandomResource(richest)
	if stolen == "" {
		return
	}
	active.Resources[stolen]++
	r.appendLogLocked(fmt.Sprintf("%s stole 1 %s from %s.", active.Name, stolen, richest.Name))
}

func (r *room) produceResourcesLocked(player *Player) {
	for i := 0; i < player.Settlements; i++ {
		player.Resources[randomResource()]++
	}
	for i := 0; i < player.Cities; i++ {
		player.Resources[randomResource()] += 2
	}
}

func (r *room) buildLocked(playerID, buildType string) error {
	if r.phase == phaseFinished {
		return errRoomFinished
	}
	if r.phase != phaseActions {
		return errRollFirst
	}
	player := r.currentPlayerLocked()
	if player == nil || player.ID != playerID {
		return errNotYourTurn
	}

	cost, ok := buildCosts[buildType]
	if !ok {
		return errors.New("unknown build type")
	}
	if !canAfford(player, cost) {
		return errors.New("not enough resources")
	}

	switch buildType {
	case "road":
		if player.Roads >= 15 {
			return errors.New("road limit reached")
		}
	case "settlement":
		if player.Settlements+player.Cities >= 5 {
			return errors.New("settlement limit reached")
		}
		if player.Roads < player.Settlements+player.Cities {
			return errors.New("build a road before placing another settlement")
		}
	case "city":
		if player.Settlements == 0 {
			return errors.New("you need a settlement to upgrade")
		}
		if player.Cities >= 4 {
			return errors.New("city limit reached")
		}
	}

	payCost(player, cost)
	switch buildType {
	case "road":
		player.Roads++
	case "settlement":
		player.Settlements++
	case "city":
		player.Settlements--
		player.Cities++
	}
	recalculateVictoryPoints(player)
	r.updatedAt = time.Now().UTC()
	r.appendLogLocked(fmt.Sprintf("%s built a %s.", player.Name, buildType))
	r.checkForWinnerLocked(player)
	return nil
}

func (r *room) tradeLocked(playerID string, give, get Resource) error {
	if r.phase == phaseFinished {
		return errRoomFinished
	}
	if r.phase != phaseActions {
		return errRollFirst
	}
	player := r.currentPlayerLocked()
	if player == nil || player.ID != playerID {
		return errNotYourTurn
	}
	if give == "" || get == "" || give == get {
		return errors.New("pick two different resources")
	}
	if player.Resources[give] < 4 {
		return errors.New("need 4 matching resources for a bank trade")
	}

	player.Resources[give] -= 4
	player.Resources[get]++
	r.updatedAt = time.Now().UTC()
	r.appendLogLocked(fmt.Sprintf("%s traded 4 %s for 1 %s.", player.Name, give, get))
	return nil
}

func (r *room) endTurnLocked(playerID string) error {
	if r.phase == phaseFinished {
		return errRoomFinished
	}
	player := r.currentPlayerLocked()
	if player == nil || player.ID != playerID {
		return errNotYourTurn
	}
	if r.phase == phaseRoll {
		return errRollFirst
	}

	r.currentTurn = (r.currentTurn + 1) % len(r.players)
	r.phase = phaseRoll
	r.lastRoll = 0
	r.turnNumber++
	r.updatedAt = time.Now().UTC()
	r.appendLogLocked(fmt.Sprintf("%s ended their turn.", player.Name))
	r.appendLogLocked(fmt.Sprintf("It is now %s's turn.", r.currentPlayerLocked().Name))
	return nil
}

func (r *room) saveLocked(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("DATA_DIR is not configured")
	}
	path, err := saveRoomSnapshot(dataDir, savedRoom{
		ID:          r.id,
		Name:        r.name,
		HostID:      r.hostID,
		Started:     r.started,
		Phase:       r.phase,
		MaxPlayers:  r.maxPlayers,
		CurrentTurn: r.currentTurn,
		LastRoll:    r.lastRoll,
		TurnNumber:  r.turnNumber,
		WinnerID:    r.winnerID,
		CreatedAt:   r.createdAt,
		UpdatedAt:   time.Now().UTC(),
		Players:     clonePlayers(r.players),
		Log:         slices.Clone(r.log),
	})
	if err != nil {
		return "", err
	}
	r.appendLogLocked(fmt.Sprintf("Game saved to %s.", path))
	r.updatedAt = time.Now().UTC()
	return path, nil
}

func (r *room) appendLogLocked(message string) {
	r.log = append(r.log, LogEntry{
		At:      time.Now().UTC(),
		Message: message,
	})
	if len(r.log) > 40 {
		r.log = r.log[len(r.log)-40:]
	}
}

func (r *room) summaryLocked() RoomSummary {
	var humans, bots int
	for _, player := range r.players {
		if player.IsAI {
			bots++
		} else {
			humans++
		}
	}
	return RoomSummary{
		ID:          r.id,
		Name:        r.name,
		Started:     r.started,
		Phase:       r.phase,
		PlayerCount: len(r.players),
		HumanCount:  humans,
		AICount:     bots,
		MaxPlayers:  r.maxPlayers,
		UpdatedAt:   r.updatedAt,
	}
}

func (r *room) stateLocked() RoomState {
	state := RoomState{
		ID:         r.id,
		Name:       r.name,
		HostID:     r.hostID,
		Started:    r.started,
		Phase:      r.phase,
		MaxPlayers: r.maxPlayers,
		LastRoll:   r.lastRoll,
		TurnNumber: r.turnNumber,
		WinnerID:   r.winnerID,
		Players:    make([]Player, 0, len(r.players)),
		Log:        slices.Clone(r.log),
		CanSave:    true,
		UpdatedAt:  r.updatedAt,
	}

	if current := r.currentPlayerLocked(); current != nil {
		state.CurrentPlayerID = current.ID
		state.CurrentPlayer = current.Name
	}

	for _, player := range r.players {
		copyPlayer := Player{
			ID:            player.ID,
			Name:          player.Name,
			IsAI:          player.IsAI,
			Connected:     player.Connected,
			Order:         player.Order,
			Resources:     map[Resource]int{},
			Roads:         player.Roads,
			Settlements:   player.Settlements,
			Cities:        player.Cities,
			VictoryPoints: player.VictoryPoints,
		}
		for _, resource := range resourceOrder {
			copyPlayer.Resources[resource] = player.Resources[resource]
		}
		state.Players = append(state.Players, copyPlayer)
		if r.winnerID == player.ID {
			state.WinnerName = player.Name
		}
	}

	return state
}

func (r *room) attachClientLocked(playerID string, client *clientConn) error {
	player := r.getPlayerLocked(playerID)
	if player == nil {
		return errors.New("player not found")
	}
	if existing := r.clients[playerID]; existing != nil {
		_ = existing.close()
	}
	r.clients[playerID] = client
	player.Connected = true
	r.updatedAt = time.Now().UTC()
	return nil
}

func (r *room) detachClientLocked(playerID string, client *clientConn) {
	current := r.clients[playerID]
	if current != client {
		return
	}
	delete(r.clients, playerID)
	if player := r.getPlayerLocked(playerID); player != nil {
		player.Connected = false
	}
	r.updatedAt = time.Now().UTC()
}

func (r *room) clientsLocked() []*clientConn {
	clients := make([]*clientConn, 0, len(r.clients))
	for _, client := range r.clients {
		clients = append(clients, client)
	}
	return clients
}

func (r *room) checkForWinnerLocked(player *Player) {
	if player.VictoryPoints < 10 {
		return
	}
	r.phase = phaseFinished
	r.winnerID = player.ID
	r.updatedAt = time.Now().UTC()
	r.appendLogLocked(fmt.Sprintf("%s won the game with %d points.", player.Name, player.VictoryPoints))
}

func sanitizePlayerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Traveler"
	}
	runes := []rune(name)
	if len(runes) > 20 {
		runes = runes[:20]
	}
	return strings.TrimSpace(string(runes))
}

func sanitizeGameName(name, hostName string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Sprintf("%s's Grantan Game", hostName)
	}
	runes := []rune(name)
	if len(runes) > 32 {
		runes = runes[:32]
	}
	return strings.TrimSpace(string(runes))
}

func clampAIPlayers(value int) int {
	if value < 0 {
		return 0
	}
	if value > 3 {
		return 3
	}
	return value
}

func recalculateVictoryPoints(player *Player) {
	player.VictoryPoints = player.Settlements + (player.Cities * 2)
}

func totalResources(player *Player) int {
	total := 0
	for _, resource := range resourceOrder {
		total += player.Resources[resource]
	}
	return total
}

func clonePlayers(players []*Player) []*Player {
	cloned := make([]*Player, 0, len(players))
	for _, player := range players {
		copyPlayer := *player
		copyPlayer.Resources = map[Resource]int{}
		for _, resource := range resourceOrder {
			copyPlayer.Resources[resource] = player.Resources[resource]
		}
		cloned = append(cloned, &copyPlayer)
	}
	return cloned
}
