export type Resource = "brick" | "lumber" | "wool" | "grain" | "ore";

export type Player = {
    id: string;
    name: string;
    isAi: boolean;
    connected: boolean;
    order: number;
    resources: Record<Resource, number>;
    roads: number;
    settlements: number;
    cities: number;
    victoryPoints: number;
};

export type LogEntry = {
    at: string;
    message: string;
};

export type RoomSummary = {
    id: string;
    name: string;
    started: boolean;
    phase: string;
    playerCount: number;
    humanCount: number;
    aiCount: number;
    maxPlayers: number;
    updatedAt: string;
};

export type RoomState = {
    id: string;
    name: string;
    hostId: string;
    started: boolean;
    phase: string;
    maxPlayers: number;
    currentPlayerId: string;
    currentPlayer: string;
    lastRoll: number;
    turnNumber: number;
    winnerId: string;
    winnerName: string;
    players: Player[];
    log: LogEntry[];
    canSave: boolean;
    updatedAt: string;
};

export type Session = {
    gameId: string;
    playerId: string;
    playerName: string;
};

export type ActionMessage =
    | { type: "roll" }
    | { type: "build"; build: "road" | "settlement" | "city" }
    | { type: "trade"; give: Resource; get: Resource }
    | { type: "end_turn" }
    | { type: "save_game" };
