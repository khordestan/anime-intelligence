# Anime Intelligence

A data-first anime tracker and taste-intelligence platform. Modular monolith: Go REST API + Next.js web app.

## Current implementation
- Go HTTP API: health, catalog search/detail, tracker CRUD, progress updates, and 1–5 ratings.
- Next.js dashboard: catalog search, add-to-library, status filters, episode progress, and ratings.
- Local JSON persistence (`data/local.json`, created automatically), seeded starter catalog.
- Input validation, request-size limits, local-origin CORS, structured request logs, graceful shutdown.
- PostgreSQL target schema in `migrations/`; not yet wired into the running API.

## Important limitations
This is a local development foundation, not production-ready multi-user software. Authentication, tenant isolation, PostgreSQL runtime integration, external provider sync, recommendations, AI analyst, and release notifications are not implemented. Do not expose the API publicly.

## Stack
Go 1.23+, standard-library HTTP server; Next.js 15, React 19, TypeScript, Tailwind CSS 4; target PostgreSQL 17. Current persistence is an atomic local JSON file.

## Run locally
API: `go run ./cmd/api` (defaults to `http://localhost:8080`).

Web:
```bash
cd web
npm install
npm run dev
```
Open `http://localhost:3000`. Configure `NEXT_PUBLIC_API_BASE_URL` when the API is elsewhere.

## API
- `GET /healthz`
- `GET /api/v1`
- `GET /api/v1/anime?q=&limit=&offset=`
- `GET /api/v1/anime/{id}`
- `GET /api/v1/tracker`
- `POST /api/v1/tracker` — `{ "animeId": "uuid", "watchStatus": "PLANNING", "episodesWatched": 0 }`
- `PATCH /api/v1/tracker/{id}` — `{ "watchStatus": "WATCHING", "episodesWatched": 3 }`
- `PUT /api/v1/ratings/{id}` — `{ "score": 5 }`

Statuses: `PLANNING`, `WATCHING`, `COMPLETED`, `ON_HOLD`, `DROPPED`.

## Layout
- `cmd/api`: API entry point
- `cmd/worker`: background worker placeholder
- `internal/httpapi`: routing and middleware
- `internal/modules/catalog`: catalog endpoints
- `internal/modules/tracker`: watchlist, progress, rating endpoints
- `internal/platform/store`: local development persistence
- `migrations`: target PostgreSQL schema
- `web`: Next.js application
- `docs`: product, architecture, data policy, roadmap, implementation log

## Data licensing
External provider integrations remain disabled until API terms and intended display, caching, persistence, commercial, and AI/ML uses have been evaluated. See `docs/DATA_SOURCE_POLICY.md`.
