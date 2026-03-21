# Tumladan Backend Skeleton

Production-like starter backend for a browser-based tabletop platform. The project is a modular monolith on Go 1.22 with HTTP API, WebSocket realtime, PostgreSQL persistence, guest auth, rooms, matches, runtime orchestration, and a stubbed `carcassonne` game module.

## Architecture

The code is split by boundaries instead of technical sprawl:

- `transport`: HTTP and WebSocket handlers, request/response DTOs, message envelopes.
- `application`: use-case level services for auth, rooms, matches, and game runtime.
- `domain`: room and match models, generic game contracts, registry, and game modules.
- `infrastructure`: PostgreSQL repositories, JWT auth, realtime hub, config, and logging.

Important design choices:

- `room` and `match` are different entities.
- The client sends only commands, never authoritative game state.
- The server loads match state, validates commands, executes game events, applies them, and persists the new state.
- New games are plugged in via `GameModule`.
- Player-specific state is prepared through `GetPlayerView`.

## Project Tree

```text
.
├── cmd/server/main.go
├── internal
│   ├── application
│   │   ├── auth
│   │   ├── matches
│   │   ├── rooms
│   │   └── runtime
│   ├── config
│   ├── domain
│   │   ├── game
│   │   │   ├── contracts
│   │   │   └── runtime
│   │   ├── games/carcassonne
│   │   ├── match
│   │   └── room
│   ├── infrastructure
│   │   ├── auth
│   │   ├── logging
│   │   ├── postgres
│   │   └── realtime
│   └── transport
│       ├── http
│       └── ws
├── migrations
├── pkg/api
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── .env.example
```

## Local Run

1. `cp .env.example .env`
2. Update `POSTGRES_DSN` in `.env` if needed.
3. `make tidy`
4. `docker compose up postgres -d`
5. `set -a && source .env && set +a && make run`

Full stack in Docker:

1. `docker compose up --build`

## HTTP API

- `POST /api/v1/auth/guest`
- `POST /api/v1/rooms`
- `GET /api/v1/rooms/public`
- `GET /api/v1/rooms/{id}`
- `POST /api/v1/matches`
- `GET /api/v1/health`
- `GET /ws?token=<jwt>`

### `POST /api/v1/auth/guest`

Request:

```json
{
  "displayName": "Meeple Fan"
}
```

Response:

```json
{
  "token": "jwt-token",
  "actor": {
    "id": "actor-id",
    "type": "guest",
    "displayName": "Meeple Fan"
  }
}
```

## WebSocket Messages

Client to server:

```json
{"type":"join_room","payload":{"roomId":"room-id","matchId":"match-id"}}
{"type":"leave_room","payload":{"roomId":"room-id","matchId":"match-id"}}
{"type":"send_chat_message","payload":{"roomId":"room-id","message":"hello"}}
{"type":"game_command","payload":{"matchId":"match-id","command":{"type":"end_turn"}}}
{"type":"ping"}
```

Server to client:

```json
{"type":"connected","payload":{"actorId":"...","displayName":"..."}}
{"type":"room_state","roomId":"room-id","payload":{...}}
{"type":"match_state","matchId":"match-id","payload":{...}}
{"type":"chat_message","roomId":"room-id","payload":{...}}
{"type":"error","payload":{"message":"..."}}
{"type":"pong"}
```

## What Is Implemented

- Guest auth with JWT token generation.
- Room creation, room lookup, public room listing.
- Match creation with initial game state.
- Game registry and runtime service.
- WebSocket connection hub with room and match subscriptions.
- Chat persistence plus room broadcast.
- Carcassonne stub with fixed turn progression and `end_turn`.
- PostgreSQL migrations and repositories.
- Minimal unit tests for registry and Carcassonne stub.

## What Is Stubbed

- Full Carcassonne rules, tile placement, scoring, and meeple logic.
- Account-based auth and authorization rules.
- Room participant lifecycle and invite-link flows.
- Redis integration.
- Rich room state snapshots and reconnect semantics.
- Per-player fan-out of distinct realtime state to every subscriber.
