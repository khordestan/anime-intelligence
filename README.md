# Anime Intelligence

A Go and Next.js foundation for a detailed anime catalog, personal tracking, and explainable recommendations.

## Implemented

- Catalog search/detail, tracker create/list/update, episode progress, and 1–5 ratings.
- Optional PostgreSQL runtime with registration, login, current account, logout, and atomic session rotation.
- Argon2id password hashes; random 256-bit bearer credentials stored only as SHA-256 digests, expiring after 24 hours.
- User-scoped queries and transaction-local forced PostgreSQL RLS; runtime rejects superuser, BYPASSRLS, and table-owner connections.
- Embedded, checksummed, transactional migration runner with advisory locking.
- Single-user JSON development mode and a Next.js dashboard. The dashboard currently supports JSON mode; PostgreSQL authentication is available through the API.
- Canonical catalog/provenance schema, 40-feature taxonomy, and deterministic Personal Match ranking core (not yet exposed through HTTP).
- CI for Go race tests, vet/build, PostgreSQL runtime/HTTP isolation, migration apply/rollback, and frontend typecheck/build; CodeQL and dependency review.

## Run locally

Go 1.26+ is required. Without `DATABASE_URL`, `go run ./cmd/api` uses the local JSON store (`DATA_FILE`, default `./data/local.json`) and listens on `127.0.0.1:8080`. Run `npm install && npm run dev` in `web` for the local dashboard.

For PostgreSQL setup, role provisioning, API usage, and verification, see [PostgreSQL runtime](docs/POSTGRES_RUNTIME.md). Migration and runtime connections must use separate database roles. Database initialization errors stop startup; the API never falls back to JSON when a non-empty `DATABASE_URL` fails.

## Development boundary

**Do not deploy publicly or use sensitive data yet.** JSON mode is single-user and unauthenticated. PostgreSQL authentication and isolation have automated coverage but still need independent security review, production abuse controls, recovery/verification flows, session management UI, and operational hardening. Browser cookie authentication is not implemented; the API accepts explicit bearer headers only. The worker, licensed provider synchronization, release alerts, and recommendation serving remain pending.

Only source code, synthetic fixtures, and permitted catalog assets belong in this public repository. Never commit secrets, real account data, personal watch histories, or database dumps.
