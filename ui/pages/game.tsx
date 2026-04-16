import Head from "next/head";
import Link from "next/link";
import { useRouter } from "next/router";
import { FormEvent, useEffect, useRef, useState } from "react";
import { createSocket, fetchGame, joinGame, saveGame, startGame } from "../grantan/api";
import { loadName, loadSession, saveName, saveSession } from "../grantan/session";
import { ActionMessage, Resource, RoomState } from "../grantan/types";

const resources: Resource[] = ["brick", "lumber", "wool", "grain", "ore"];

function totalResources(player: RoomState["players"][number]): number {
    return resources.reduce((sum, resource) => sum + player.resources[resource], 0);
}

export default function GamePage() {
    const router = useRouter();
    const socketRef = useRef<WebSocket | null>(null);

    const [gameId, setGameId] = useState("");
    const [playerId, setPlayerId] = useState("");
    const [playerName, setPlayerName] = useState("");
    const [state, setState] = useState<RoomState | null>(null);
    const [status, setStatus] = useState("Loading room...");
    const [error, setError] = useState("");
    const [give, setGive] = useState<Resource>("brick");
    const [get, setGet] = useState<Resource>("grain");
    const [saveMessage, setSaveMessage] = useState("");

    useEffect(() => {
        if (!router.isReady) {
            return;
        }

        const rawGame = router.query.game;
        const rawPlayer = router.query.player;
        const nextGame = typeof rawGame === "string" ? rawGame.toUpperCase() : "";
        const nextPlayer = typeof rawPlayer === "string" ? rawPlayer : "";
        const fallbackName = loadName();

        setGameId(nextGame);
        setPlayerName(fallbackName);

        if (nextGame && !nextPlayer) {
            const session = loadSession(nextGame);
            if (session) {
                setPlayerId(session.playerId);
                setPlayerName(session.playerName);
                void router.replace(`/game?game=${session.gameId}&player=${session.playerId}`, undefined, { shallow: true });
                return;
            }
        }

        setPlayerId(nextPlayer);
    }, [router]);

    useEffect(() => {
        if (!gameId) {
            return;
        }

        let cancelled = false;
        setStatus("Loading room...");

        void fetchGame(gameId)
            .then((room) => {
                if (cancelled) {
                    return;
                }
                setState(room);
                if (!playerName && playerId) {
                    const me = room.players.find((player) => player.id === playerId);
                    if (me) {
                        setPlayerName(me.name);
                    }
                }
                setStatus(playerId ? "Connecting..." : "Waiting for you to join...");
            })
            .catch((nextError) => {
                if (!cancelled) {
                    setError(nextError instanceof Error ? nextError.message : "Could not load room.");
                    setStatus("Room unavailable");
                }
            });

        return () => {
            cancelled = true;
        };
    }, [gameId, playerId, playerName]);

    useEffect(() => {
        if (!gameId || !playerId) {
            return;
        }

        const socket = createSocket(gameId, playerId);
        socketRef.current = socket;
        setStatus("Connecting...");

        socket.onopen = () => {
            setStatus("Connected");
            setError("");
            if (playerName) {
                saveSession({ gameId, playerId, playerName });
            }
        };

        socket.onmessage = (event) => {
            const message = JSON.parse(event.data) as { type: string; state?: RoomState; error?: string };
            if (message.type === "state" && message.state) {
                setState(message.state);
                const me = message.state.players.find((player) => player.id === playerId);
                if (me) {
                    setPlayerName(me.name);
                    saveSession({ gameId, playerId, playerName: me.name });
                }
            }
            if (message.type === "error" && message.error) {
                setError(message.error);
            }
        };

        socket.onclose = () => {
            setStatus("Disconnected");
        };

        socket.onerror = () => {
            setStatus("Socket error");
        };

        return () => {
            socket.close();
            socketRef.current = null;
        };
    }, [gameId, playerId, playerName]);

    async function handleJoin(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        if (!gameId) {
            return;
        }

        try {
            saveName(playerName);
            const response = await joinGame(gameId, playerName);
            saveSession({
                gameId: response.gameId,
                playerId: response.playerId,
                playerName,
            });
            setPlayerId(response.playerId);
            await router.replace(`/game?game=${response.gameId}&player=${response.playerId}`, undefined, { shallow: true });
        } catch (nextError) {
            setError(nextError instanceof Error ? nextError.message : "Could not join room.");
        }
    }

    function sendAction(action: ActionMessage) {
        if (!socketRef.current || socketRef.current.readyState !== WebSocket.OPEN) {
            setError("WebSocket is not connected.");
            return;
        }
        socketRef.current.send(JSON.stringify(action));
    }

    async function handleStart() {
        if (!gameId || !playerId) {
            return;
        }

        try {
            const nextState = await startGame(gameId, playerId);
            setState(nextState);
        } catch (nextError) {
            setError(nextError instanceof Error ? nextError.message : "Could not start game.");
        }
    }

    async function handleSave() {
        if (!gameId || !playerId) {
            return;
        }

        try {
            const savedTo = await saveGame(gameId, playerId);
            setSaveMessage(`Saved to ${savedTo}`);
        } catch (nextError) {
            setError(nextError instanceof Error ? nextError.message : "Could not save game.");
        }
    }

    const me = state?.players.find((player) => player.id === playerId) || null;
    const isHost = state?.hostId === playerId;
    const isMyTurn = state?.currentPlayerId === playerId;

    if (!gameId) {
        return (
            <main className="shell">
                <p className="error">No game selected.</p>
                <Link href="/">Return to lobby</Link>
            </main>
        );
    }

    return (
        <>
            <Head>
                <title>{state ? `${state.name} | Grantan` : "Grantan"}</title>
            </Head>

            <main className="shell">
                <section className="hero compact">
                    <div>
                        <p className="eyebrow">Room {gameId}</p>
                        <h1>{state?.name || "Grantan"}</h1>
                        <p className="subtitle">
                            Share <code>{`/game?game=${gameId}`}</code>{" "}
                            so friends can join before you start.
                        </p>
                    </div>

                    <div className="hero-actions">
                        <Link className="link-button" href="/">
                            Back to lobby
                        </Link>
                        <span className="status-pill">{status}</span>
                    </div>
                </section>

                {error ? <p className="error">{error}</p> : null}
                {saveMessage ? <p className="success">{saveMessage}</p> : null}

                {!playerId ? (
                    <section className="panel">
                        <h2>Join this room</h2>
                        <form className="stack" onSubmit={handleJoin}>
                            <label className="field">
                                <span>Your name</span>
                                <input
                                    value={playerName}
                                    onChange={(event) => setPlayerName(event.target.value)}
                                    maxLength={20}
                                    required
                                />
                            </label>

                            <button className="primary" disabled={!playerName.trim()} type="submit">
                                Join game
                            </button>
                        </form>
                    </section>
                ) : null}

                <section className="grid game-layout">
                    <div className="panel">
                        <div className="section-head">
                            <h2>Players</h2>
                            <p>
                                {state?.started
                                    ? `${state.currentPlayer || "Waiting"} is up.`
                                    : "Lobby is open until the host starts."}
                            </p>
                        </div>

                        <div className="player-list">
                            {state?.players.map((player) => (
                                <article className={`player-card ${player.id === state.currentPlayerId ? "current" : ""}`} key={player.id}>
                                    <div className="player-top">
                                        <div>
                                            <p className="game-title">{player.name}</p>
                                            <p className="muted">
                                                {player.isAi ? "AI player" : "Human player"} - {player.connected ? "Online" : "Offline"}
                                            </p>
                                        </div>
                                        <span className="vp-badge">{player.victoryPoints} VP</span>
                                    </div>

                                    <div className="stats-grid">
                                        <span>Roads {player.roads}</span>
                                        <span>Settlements {player.settlements}</span>
                                        <span>Cities {player.cities}</span>
                                        <span>Cards {totalResources(player)}</span>
                                    </div>

                                    <div className="resource-row">
                                        {resources.map((resource) => (
                                            <span key={resource}>
                                                {resource}: {player.resources[resource]}
                                            </span>
                                        ))}
                                    </div>
                                </article>
                            ))}
                        </div>
                    </div>

                    <div className="panel">
                        <div className="section-head">
                            <h2>Turn controls</h2>
                            <p>
                                {state?.winnerName
                                    ? `${state.winnerName} has won the match.`
                                    : state?.started
                                      ? `Turn ${state.turnNumber} - Last roll ${state.lastRoll || "-"}`
                                      : "Configure players, then start when ready."}
                            </p>
                        </div>

                        <div className="actions-grid">
                            {!state?.started ? (
                                <button className="primary" disabled={!isHost || (state?.players.length || 0) < 2} onClick={() => void handleStart()}>
                                    Start game
                                </button>
                            ) : null}

                            <button
                                className="primary"
                                disabled={!isMyTurn || state?.phase !== "roll" || me?.isAi}
                                onClick={() => sendAction({ type: "roll" })}
                            >
                                Roll dice
                            </button>

                            <button
                                className="secondary"
                                disabled={!isMyTurn || state?.phase !== "actions" || me?.isAi}
                                onClick={() => sendAction({ type: "build", build: "road" })}
                            >
                                Build road
                            </button>

                            <button
                                className="secondary"
                                disabled={!isMyTurn || state?.phase !== "actions" || me?.isAi}
                                onClick={() => sendAction({ type: "build", build: "settlement" })}
                            >
                                Build settlement
                            </button>

                            <button
                                className="secondary"
                                disabled={!isMyTurn || state?.phase !== "actions" || me?.isAi}
                                onClick={() => sendAction({ type: "build", build: "city" })}
                            >
                                Build city
                            </button>

                            <button className="secondary" disabled={!isHost} onClick={() => void handleSave()}>
                                Save JSON
                            </button>

                            <button
                                className="primary"
                                disabled={!isMyTurn || state?.phase !== "actions" || me?.isAi}
                                onClick={() => sendAction({ type: "end_turn" })}
                            >
                                End turn
                            </button>
                        </div>

                        <div className="trade-box">
                            <h3>Bank trade</h3>
                            <div className="trade-row">
                                <select value={give} onChange={(event) => setGive(event.target.value as Resource)}>
                                    {resources.map((resource) => (
                                        <option key={resource} value={resource}>
                                            Give {resource}
                                        </option>
                                    ))}
                                </select>
                                <select value={get} onChange={(event) => setGet(event.target.value as Resource)}>
                                    {resources.map((resource) => (
                                        <option key={resource} value={resource}>
                                            Get {resource}
                                        </option>
                                    ))}
                                </select>
                                <button
                                    className="secondary"
                                    disabled={!isMyTurn || state?.phase !== "actions" || me?.isAi || give === get}
                                    onClick={() => sendAction({ type: "trade", give, get })}
                                >
                                    Trade 4:1
                                </button>
                            </div>
                            <p className="muted">AI players will trade only when it helps them finish a build.</p>
                        </div>
                    </div>
                </section>

                <section className="panel">
                    <div className="section-head">
                        <h2>Match log</h2>
                        <p>The server is authoritative, and every state update is pushed over WebSocket.</p>
                    </div>

                    <div className="log-list">
                        {state?.log.slice().reverse().map((entry, index) => (
                            <p key={`${entry.at}-${index}`}>
                                <span className="muted">{new Date(entry.at).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}</span>{" "}
                                {entry.message}
                            </p>
                        ))}
                    </div>
                </section>
            </main>
        </>
    );
}
