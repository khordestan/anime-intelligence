# Anime Intelligence

A data-first anime tracker and taste-intelligence platform. The initial architecture is a Go modular monolith with a Next.js web client.

## Current implementation

- Go HTTP API: health, catalog search/detail, tracker create/list/update, progress, and 1–5 ratings.
- Next.js dashboard: starter-catalog search, add-to-library, status filters, episode progress, and ratings.
- Local JSON persistence for the runnable single-user development slice.
- PostgreSQL canonical schema with users, catalog, source provenance, feature observations, tracker, ratings, preferences, recommendations, schedules, events, and notifications.
- Deterministic Personal Match ranking core with separate explicit/inferred preferences, evidence output, exclusion filters, and unit tests (not yet exposed through the API).
- Provider-neutral catalog fixtures and a curated 40-feature taxonomy across protagonist, power system, combat, narrative, theme, setting, pacing, tone, stakes, content, and ending.
- GitHub Actions CI: Go tests (including race detector), vet/build, frontend typecheck/build, and PostgreSQL migration/rollback checks.
- Security automation: CodeQL analysis for Go and TypeScript/JavaScript, dependency review on pull requests, and weekly Dependabot update proposals.

## Development status and limitations

This is a development foundation, not a production-ready multi-user service. The running API currently uses a local JSON store; PostgreSQL migrations are authored but are not wired into the API runtime. Authentication, sessions, authorization, enforced tenant isolation, external provider synchronization, recommendation serving, AI analysis, and release notifications are not implemented.

**Do not expose the API publicly or use it for sensitive data.** The current local JSON store is single-user development infrastructure, not a secure account system.

## Public repository and privacy

This repository is intentionally public. Public source code must contain only synthetic fixtures and non-personal examples.

- Never commit real user watch histories, ratings, notes, preferences, email addresses, or account data.
- Never commit API keys, passwords, tokens, private keys, production logs, database dumps, or real `.env` files.
- Use `.env.example` only for clearly fake placeholders.
- Provider API access does not grant permission to persist, redistribute, display, commercially exploit, or use provider data for AI/ML. Review terms before integration.
- Do not add artwork, synopsis text, or other third-party assets unless the applicable license permits the intended use.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md) before contributing or reporting a vulnerability.

## Stack

- Go 1.23+, standard-library HTTP server
- Next.js 15, React 19, TypeScript, Tailwind CSS 4
- Target database: PostgreSQL 17
- Current development persistence: atomic local JSON file

## Run locally

API:
```bash
go run ./cmd/api
```
The API listens on `http://localhost:8080` by default.

Web:
```bash
cd web
npm install
npm run dev
```
Open `http://localhost:3000`. Set `NEXT_PUBLIC_API_BASE_URL` if the API is elsewhere.

## Database schema

Migrations are ordered:
1. `000001_initial_schema` — canonical entities and user-scoped product tables.
2. `000002_catalog_seed` — provider-neutral starter catalog.
3. `000003_feature_taxonomy` — curated product taxonomy.

They are SQL migration files only; no migration runner or live PostgreSQL integration is wired yet.

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
- `migrations`: PostgreSQL schema and seed migrations
- `web`: Next.js application
- `docs`: product, architecture, data policy, taxonomy, roadmap, implementation log

## Data licensing

External provider integrations remain disabled until API terms and intended display, caching, persistence, commercial, and AI/ML uses have been evaluated. API access is not treated as a license to persist or redistribute data. See `docs/DATA_SOURCE_POLICY.md`.
