package grantan

import "time"

type Resource string

const (
	ResourceBrick  Resource = "brick"
	ResourceLumber Resource = "lumber"
	ResourceWool   Resource = "wool"
	ResourceGrain  Resource = "grain"
	ResourceOre    Resource = "ore"
)

var resourceOrder = []Resource{
	ResourceBrick,
	ResourceLumber,
	ResourceWool,
	ResourceGrain,
	ResourceOre,
}

const (
	phaseLobby    = "lobby"
	phaseRoll     = "roll"
	phaseActions  = "actions"
	phaseFinished = "finished"
)

type Player struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	IsAI          bool             `json:"isAi"`
	Connected     bool             `json:"connected"`
	Order         int              `json:"order"`
	Resources     map[Resource]int `json:"resources"`
	Roads         int              `json:"roads"`
	Settlements   int              `json:"settlements"`
	Cities        int              `json:"cities"`
	VictoryPoints int              `json:"victoryPoints"`
}

type LogEntry struct {
	At      time.Time `json:"at"`
	Message string    `json:"message"`
}

type RoomSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Started     bool      `json:"started"`
	Phase       string    `json:"phase"`
	PlayerCount int       `json:"playerCount"`
	HumanCount  int       `json:"humanCount"`
	AICount     int       `json:"aiCount"`
	MaxPlayers  int       `json:"maxPlayers"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type RoomState struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	HostID          string     `json:"hostId"`
	Started         bool       `json:"started"`
	Phase           string     `json:"phase"`
	MaxPlayers      int        `json:"maxPlayers"`
	CurrentPlayerID string     `json:"currentPlayerId"`
	CurrentPlayer   string     `json:"currentPlayer"`
	LastRoll        int        `json:"lastRoll"`
	TurnNumber      int        `json:"turnNumber"`
	WinnerID        string     `json:"winnerId"`
	WinnerName      string     `json:"winnerName"`
	Players         []Player   `json:"players"`
	Log             []LogEntry `json:"log"`
	CanSave         bool       `json:"canSave"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type savedRoom struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	HostID      string     `json:"hostId"`
	Started     bool       `json:"started"`
	Phase       string     `json:"phase"`
	MaxPlayers  int        `json:"maxPlayers"`
	CurrentTurn int        `json:"currentTurn"`
	LastRoll    int        `json:"lastRoll"`
	TurnNumber  int        `json:"turnNumber"`
	WinnerID    string     `json:"winnerId"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Players     []*Player  `json:"players"`
	Log         []LogEntry `json:"log"`
}

type createGameRequest struct {
	PlayerName string `json:"playerName"`
	GameName   string `json:"gameName"`
	AIPlayers  int    `json:"aiPlayers"`
}

type joinGameRequest struct {
	PlayerName string `json:"playerName"`
}

type actorRequest struct {
	PlayerID string `json:"playerId"`
}

type wsAction struct {
	Type  string   `json:"type"`
	Build string   `json:"build,omitempty"`
	Give  Resource `json:"give,omitempty"`
	Get   Resource `json:"get,omitempty"`
}

type wsEnvelope struct {
	Type  string     `json:"type"`
	State *RoomState `json:"state,omitempty"`
	Error string     `json:"error,omitempty"`
}
