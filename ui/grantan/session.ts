import { Session } from "./types";

const nameKey = "grantan:name";

function sessionKey(gameId: string): string {
    return `grantan:session:${gameId}`;
}

export function saveName(name: string): void {
    if (typeof window === "undefined") {
        return;
    }
    window.localStorage.setItem(nameKey, name);
}

export function loadName(): string {
    if (typeof window === "undefined") {
        return "";
    }
    return window.localStorage.getItem(nameKey) || "";
}

export function saveSession(session: Session): void {
    if (typeof window === "undefined") {
        return;
    }
    saveName(session.playerName);
    window.localStorage.setItem(sessionKey(session.gameId), JSON.stringify(session));
}

export function loadSession(gameId: string): Session | null {
    if (typeof window === "undefined") {
        return null;
    }
    const raw = window.localStorage.getItem(sessionKey(gameId));
    if (!raw) {
        return null;
    }

    try {
        return JSON.parse(raw) as Session;
    } catch {
        return null;
    }
}
