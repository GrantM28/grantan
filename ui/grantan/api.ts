import { RoomState, RoomSummary } from "./types";

type CreateGameResponse = {
    gameId: string;
    playerId: string;
    state: RoomState;
};

type JoinGameResponse = {
    gameId: string;
    playerId: string;
    state: RoomState;
};

async function request<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
    const response = await fetch(input, {
        ...init,
        headers: {
            "Content-Type": "application/json",
            ...(init?.headers || {}),
        },
    });

    if (!response.ok) {
        const body = (await response.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error || "Request failed");
    }

    return (await response.json()) as T;
}

export async function listGames(): Promise<RoomSummary[]> {
    const response = await request<{ games: RoomSummary[] }>("/api/games");
    return response.games;
}

export async function fetchGame(gameId: string): Promise<RoomState> {
    return request<RoomState>(`/api/games/${gameId}`);
}

export async function createGame(input: {
    playerName: string;
    gameName: string;
    aiPlayers: number;
}): Promise<CreateGameResponse> {
    return request<CreateGameResponse>("/api/games", {
        method: "POST",
        body: JSON.stringify(input),
    });
}

export async function joinGame(gameId: string, playerName: string): Promise<JoinGameResponse> {
    return request<JoinGameResponse>(`/api/games/${gameId}/join`, {
        method: "POST",
        body: JSON.stringify({ playerName }),
    });
}

export async function startGame(gameId: string, playerId: string): Promise<RoomState> {
    const response = await request<{ state: RoomState }>(`/api/games/${gameId}/start`, {
        method: "POST",
        body: JSON.stringify({ playerId }),
    });
    return response.state;
}

export async function saveGame(gameId: string, playerId: string): Promise<string> {
    const response = await request<{ savedTo: string }>(`/api/games/${gameId}/save`, {
        method: "POST",
        body: JSON.stringify({ playerId }),
    });
    return response.savedTo;
}

export function createSocket(gameId: string, playerId: string): WebSocket {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = new URL(`${protocol}//${window.location.host}/ws`);
    url.searchParams.set("gameId", gameId);
    url.searchParams.set("playerId", playerId);
    return new WebSocket(url);
}
