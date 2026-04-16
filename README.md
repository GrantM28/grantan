# Grantan

Grantan is a simplified, self-contained multiplayer web game inspired by Catan. It runs as a single Docker container, keeps game state in memory, supports browser multiplayer over WebSockets, and includes basic AI players for mixed human/CPU matches.

## What changed

- MongoDB was removed completely.
- The Go server is now the authoritative in-memory game server.
- The React UI in [`ui`](./ui) talks directly to the Go backend with REST + WebSocket.
- Optional manual saves write JSON snapshots to `/data/games/<room>.json`.
- Docker now builds the frontend, builds the Go binary, and serves everything from one container.

## Updated structure

```text
.
|-- cmd/server/main.go
|-- grantan/
|   |-- ai.go
|   |-- persistence.go
|   |-- random.go
|   |-- room.go
|   `-- server.go
|-- ui/
|   |-- grantan/
|   |   |-- api.ts
|   |   |-- session.ts
|   |   `-- types.ts
|   |-- pages/
|   |   |-- _app.tsx
|   |   |-- game.tsx
|   |   `-- index.tsx
|   `-- styles/globals.css
|-- Dockerfile
|-- docker-compose.yml
`-- data/
    `-- .gitkeep
```

## Run locally

### Docker

```bash
docker compose up -d --build
```

Then open [http://localhost:5678](http://localhost:5678).

### Native dev

Backend:

```bash
go run ./cmd/server
```

Frontend:

```bash
cd ui
npm install
npm run dev
```

The Next app expects the Go server on the same origin in production. For local frontend dev, proxy through your browser or run it separately on port 3000 while the backend stays on 5678.

## Unraid / reverse proxy

1. Clone the repo onto your server.
2. From the repo root, run `docker compose up -d --build`.
3. Point your reverse proxy at `http://<server-ip>:5678`.
4. Keep WebSocket upgrades enabled for `/ws`.
5. Mount `./data:/data` if you want manual JSON saves to persist across container restarts.

Because the frontend derives its API and WebSocket URLs from `window.location`, it works behind HTTP, HTTPS, Nginx, Traefik, or Unraid reverse proxies without hardcoded hostnames.

## Gameplay notes

- Create a lobby with 0-3 AI players.
- Have friends join before the host starts the match.
- Each turn goes: roll -> optional build/trade -> end turn.
- AI players wait about 1-2 seconds before acting.
- The rules are intentionally simplified so the server stays portable and easy to maintain.

## Persistence

- Running games live only in memory.
- Restarting the container clears active rooms.
- The host can manually save a room to JSON when `DATA_DIR` is available.

## License

This repository remains under the AGPLv3 license. Existing artwork and media files remain subject to their original ownership and license terms.
