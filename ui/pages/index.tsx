import Head from "next/head";
import { useRouter } from "next/router";
import { FormEvent, useEffect, useState } from "react";
import { createGame, joinGame, listGames } from "../grantan/api";
import { loadName, saveName, saveSession } from "../grantan/session";
import { RoomSummary } from "../grantan/types";

function formatUpdatedAt(value: string): string {
    return new Date(value).toLocaleTimeString([], {
        hour: "numeric",
        minute: "2-digit",
    });
}

export default function LobbyPage() {
    const router = useRouter();
    const [playerName, setPlayerName] = useState("");
    const [gameName, setGameName] = useState("");
    const [joinCode, setJoinCode] = useState("");
    const [aiPlayers, setAiPlayers] = useState(2);
    const [games, setGames] = useState<RoomSummary[]>([]);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");

    useEffect(() => {
        setPlayerName(loadName());
    }, []);

    useEffect(() => {
        let cancelled = false;

        async function refresh() {
            try {
                const nextGames = await listGames();
                if (!cancelled) {
                    setGames(nextGames);
                }
            } catch (nextError) {
                if (!cancelled) {
                    setError(nextError instanceof Error ? nextError.message : "Could not load games.");
                }
            }
        }

        void refresh();
        const interval = window.setInterval(() => void refresh(), 5000);
        return () => {
            cancelled = true;
            window.clearInterval(interval);
        };
    }, []);

    async function handleCreate(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        setBusy(true);
        setError("");

        try {
            saveName(playerName);
            const response = await createGame({
                playerName,
                gameName,
                aiPlayers,
            });

            saveSession({
                gameId: response.gameId,
                playerId: response.playerId,
                playerName,
            });

            await router.push(`/game?game=${response.gameId}&player=${response.playerId}`);
        } catch (nextError) {
            setError(nextError instanceof Error ? nextError.message : "Could not create game.");
        } finally {
            setBusy(false);
        }
    }

    async function handleJoin(gameId: string) {
        setBusy(true);
        setError("");

        try {
            saveName(playerName);
            const response = await joinGame(gameId.toUpperCase(), playerName);
            saveSession({
                gameId: response.gameId,
                playerId: response.playerId,
                playerName,
            });
            await router.push(`/game?game=${response.gameId}&player=${response.playerId}`);
        } catch (nextError) {
            setError(nextError instanceof Error ? nextError.message : "Could not join game.");
        } finally {
            setBusy(false);
        }
    }

    return (
        <>
            <Head>
                <title>Grantan</title>
            </Head>

            <main className="shell">
                <section className="hero">
                    <p className="eyebrow">Single-container multiplayer</p>
                    <h1>Grantan</h1>
                    <p className="subtitle">
                        A simplified Catan-like game with browser multiplayer, built-in AI, and no database.
                    </p>
                </section>

                <section className="grid two-up">
                    <div className="panel">
                        <h2>Create a game</h2>
                        <form className="stack" onSubmit={handleCreate}>
                            <label className="field">
                                <span>Your name</span>
                                <input
                                    value={playerName}
                                    onChange={(event) => setPlayerName(event.target.value)}
                                    placeholder="Morgan"
                                    maxLength={20}
                                    required
                                />
                            </label>

                            <label className="field">
                                <span>Game name</span>
                                <input
                                    value={gameName}
                                    onChange={(event) => setGameName(event.target.value)}
                                    placeholder="Friday night match"
                                    maxLength={32}
                                />
                            </label>

                            <label className="field">
                                <span>AI players</span>
                                <select value={aiPlayers} onChange={(event) => setAiPlayers(Number(event.target.value))}>
                                    <option value={0}>0 AI</option>
                                    <option value={1}>1 AI</option>
                                    <option value={2}>2 AI</option>
                                    <option value={3}>3 AI</option>
                                </select>
                            </label>

                            <button className="primary" disabled={busy || !playerName.trim()} type="submit">
                                {busy ? "Working..." : "Create lobby"}
                            </button>
                        </form>
                    </div>

                    <div className="panel">
                        <h2>Join by code</h2>
                        <div className="stack">
                            <label className="field">
                                <span>Lobby code</span>
                                <input
                                    value={joinCode}
                                    onChange={(event) => setJoinCode(event.target.value.toUpperCase())}
                                    placeholder="ABCD"
                                    maxLength={4}
                                />
                            </label>

                            <button
                                className="secondary"
                                disabled={busy || !playerName.trim() || joinCode.trim().length < 4}
                                onClick={() => void handleJoin(joinCode)}
                                type="button"
                            >
                                Join lobby
                            </button>
                        </div>
                    </div>
                </section>

                <section className="panel">
                    <div className="section-head">
                        <h2>Open lobbies</h2>
                        <p>Share a link or have friends join by code before the host starts.</p>
                    </div>

                    {error ? <p className="error">{error}</p> : null}

                    <div className="game-list">
                        {games.length === 0 ? <p className="muted">No lobbies yet. Start one above.</p> : null}

                        {games.map((game) => (
                            <article className="game-card" key={game.id}>
                                <div>
                                    <p className="game-title">{game.name}</p>
                                    <p className="muted">
                                        {game.id} • {game.humanCount} human / {game.aiCount} AI • {game.playerCount}/
                                        {game.maxPlayers} seats
                                    </p>
                                    <p className="muted">
                                        {game.started ? `In progress (${game.phase})` : "Waiting in lobby"} • Updated{" "}
                                        {formatUpdatedAt(game.updatedAt)}
                                    </p>
                                </div>

                                <button
                                    className="secondary"
                                    disabled={busy || game.started || game.playerCount >= game.maxPlayers || !playerName.trim()}
                                    onClick={() => void handleJoin(game.id)}
                                    type="button"
                                >
                                    {game.started ? "Running" : "Join"}
                                </button>
                            </article>
                        ))}
                    </div>
                </section>

                <section className="footnote">
                    <p>
                        Manual saves write JSON files to <code>/data/games</code> when <code>DATA_DIR</code> is mounted.
                    </p>
                </section>
            </main>
        </>
    );
}
