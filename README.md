# Anime Intelligence

A Go and Next.js foundation for a detailed anime catalog, personal tracking, and explainable recommendations.

For current implementation state, security constraints and continuation steps, start with [project context](docs/PROJECT_CONTEXT.md).

## Implemented

- Sourced catalog details, genres/themes/demographics/tags/studios, canonical season relations and per-track episode schedules.
- Reviewed offline provider ingestion with external ID mapping, latest structured provenance, schedule revision history and unique release/announcement events; offline Jikan v4 conversion and bounded CC0 Wikidata factual fetching. Scheduled collection remains disabled; comprehensive availability sources still require approval.
- Tracker create/list/update/remove, aggregate or exact episode selections (including gaps), 1–5 ratings and a dashboard episode editor.
- Verified PostgreSQL private notes with conditional saves, completed-pass archives, explicit rewatches and guarded current-pass edits, with dashboard controls and account isolation.
- Individual-season or linked-franchise follows, release-track preferences and owner-scoped in-app episode/season-announcement notifications, polled from the dashboard.
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
- Canonical catalog/provenance schema, 40-feature taxonomy, append-only reviewed evidence import, and verified-owner Personal Match API/dashboard with declared and rating-derived signals.
- CI for Go race tests, vet/build, PostgreSQL runtime/HTTP isolation, migration apply/rollback, frontend transport/session tests, mocked and real-PostgreSQL browser tests, and frontend typecheck/build; CodeQL and dependency review.

Public source freshness and latest schedule counts are available through `go run ./cmd/catalog-status`; see [catalog operations](docs/CATALOG_OPERATIONS.md). Selected CC0 facts can be fetched with `cmd/catalog-fetch` before reviewed import; this does not enable scheduled availability collection.

Operator queue counts, outcomes and age are available through `go run ./cmd/identity-status` (JSON) or `-format=prometheus` with a restricted database connection; see [queue operations](docs/IDENTITY_QUEUE_OPERATIONS.md) and [disabled monitoring examples](deployments/monitoring/README.md).

API probes: `/healthz` checks process liveness; `/readyz` checks live runtime dependencies with a bounded, generic response. Readiness does not approve public deployment. Request logs use route templates rather than raw URL paths.

## Run locally

Go 1.26+ is required. Without `DATABASE_URL`, `go run ./cmd/api` uses the local JSON store (`DATA_FILE`, default `./data/local.json`) and listens on `127.0.0.1:8080`. Run `npm ci && npm run dev` in `web` for the local dashboard.

For PostgreSQL setup, role provisioning, API usage, and verification, see [PostgreSQL runtime](docs/POSTGRES_RUNTIME.md). Migration and runtime connections must use separate database roles. Database initialization errors stop startup; the API never falls back to JSON when a non-empty `DATABASE_URL` fails.

The [backup recovery guide](docs/BACKUP_RECOVERY.md) includes a synthetic logical-restore CI drill and an offline recovery fence. Restored historical credentials and account changes must be reconciled before reopening; no backup service or deployment is activated.

## Development boundary

**Do not deploy publicly or use sensitive data yet.** JSON mode is single-user and unauthenticated. PostgreSQL authentication and isolation have automated coverage but still need independent security review, production abuse controls, operational mail/key configuration, queue monitoring, and operational hardening. Browser cookie authentication is not implemented; the API accepts explicit bearer headers only. The bounded Wikidata fact worker is opt-in and idle by default; real source activation, broad licensed metadata/availability coverage, durable per-title scheduling, background delivery channels and hybrid AI remain pending. See [catalog ingestion](docs/CATALOG_FEEDS.md), [provider approval registry](docs/PROVIDERS.md), [episode progress](docs/EPISODE_PROGRESS.md) and [release alerts](docs/RELEASE_ALERTS.md).

Only source code, synthetic fixtures, and permitted catalog assets belong in this public repository. Never commit secrets, real account data, personal watch histories, or database dumps.
