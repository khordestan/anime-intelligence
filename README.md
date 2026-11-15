# Anime Intelligence

A Go and Next.js foundation for a detailed anime catalog, personal tracking, and explainable recommendations.

For current implementation state, security constraints and continuation steps, start with [project context](docs/PROJECT_CONTEXT.md).

## Implemented

- Catalog search/detail, tracker create/list/update, episode progress, and 1–5 ratings.
- Optional PostgreSQL runtime with registration, login, current account, logout, and atomic session rotation.
- Active-session listing, individual remote revocation, and sign-out of every other session; refresh keeps a stable logical session ID.
- Optional SMTP-backed password recovery and email verification use short-lived, single-use hashed challenges, explicit confirmation, and fragment-only mail links. Delivery is disabled without explicit SMTP and outbox-key configuration; accepted requests and issued messages use encrypted, leased PostgreSQL inbox/outbox queues with a bounded decryption keyring for planned rotation.
- Password changes require the current password and atomically invalidate every existing session, including the caller.
- Account-data export from a consistent snapshot and password-confirmed deletion with session/data cascades, available from the signed-in dashboard.
- Persisted password-failure budgets shared by login, password changes and deletion, with a bounded observation window and temporary account locks; responses remain generic.
- Argon2id password hashes; random 256-bit bearer credentials stored only as SHA-256 digests, expiring after 24 hours.
- PostgreSQL tracker/rating access requires verified email; unverified users retain account, recovery and data controls.
- User-scoped queries and transaction-local forced PostgreSQL RLS; runtime rejects superuser, BYPASSRLS, and table-owner connections.
- Embedded, checksummed, transactional migration runner with advisory locking.
- Single-user JSON development mode and a Next.js dashboard with PostgreSQL login/registration, logout, session expiry, and account-switch isolation. Browser tokens stay only in memory. Signed-in users can inspect active sessions, revoke a selected login, or sign out all other sessions.
- Canonical catalog/provenance schema, 40-feature taxonomy, and deterministic Personal Match ranking core (not yet exposed through HTTP).
- CI for Go race tests, vet/build, PostgreSQL runtime/HTTP isolation, migration apply/rollback, frontend transport/session tests, mocked and real-PostgreSQL browser tests, and frontend typecheck/build; CodeQL and dependency review.

Operator queue counts, outcomes and age are available through `go run ./cmd/identity-status` (JSON) or `-format=prometheus` with a restricted database connection; see [queue operations](docs/IDENTITY_QUEUE_OPERATIONS.md) and [disabled monitoring examples](deployments/monitoring/README.md).

## Run locally

Go 1.26+ is required. Without `DATABASE_URL`, `go run ./cmd/api` uses the local JSON store (`DATA_FILE`, default `./data/local.json`) and listens on `127.0.0.1:8080`. Run `npm install && npm run dev` in `web` for the local dashboard.

For PostgreSQL setup, role provisioning, API usage, and verification, see [PostgreSQL runtime](docs/POSTGRES_RUNTIME.md). Migration and runtime connections must use separate database roles. Database initialization errors stop startup; the API never falls back to JSON when a non-empty `DATABASE_URL` fails.

## Development boundary

**Do not deploy publicly or use sensitive data yet.** JSON mode is single-user and unauthenticated. PostgreSQL authentication and isolation have automated coverage but still need independent security review, production abuse controls, operational mail/key configuration, queue monitoring, and operational hardening. Browser cookie authentication is not implemented; the API accepts explicit bearer headers only. The worker, licensed provider synchronization, release alerts, and recommendation serving remain pending.

Only source code, synthetic fixtures, and permitted catalog assets belong in this public repository. Never commit secrets, real account data, personal watch histories, or database dumps.
