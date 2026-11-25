# Anime Intelligence

A Go and Next.js foundation for a detailed anime catalog, personal tracking, and explainable recommendations.

Start with [PROGRESS.md](PROGRESS.md) for the verified checkpoint, [ROADMAP.md](ROADMAP.md) for product scope and [AGENTS.md](AGENTS.md) for development rules. See [architecture](docs/ARCHITECTURE.md) and the [API contract](api/openapi.yaml).

## Implemented

- Sourced catalog details, genres/themes/demographics/tags/studios, canonical season relations and per-track episode schedules; [bounded linked-title browsing](docs/CATALOG_CONTINUITY.md) exposes transitive prequel/sequel connections and source labels.
- Reviewed offline provider ingestion with external ID mapping, latest structured provenance, schedule revision history and unique release/announcement events; offline Jikan v4 conversion and bounded CC0 Wikidata factual fetching. Scheduled collection remains disabled; comprehensive availability sources still require approval.
- Complete transactional catalog-import previews, optional selected-metadata observation archives and a restricted read-only field-change history command; historical storage remains a separate opt-in rights/retention decision. See [catalog history](docs/CATALOG_HISTORY.md).
- Combined [catalog metadata filters](docs/CATALOG_FILTERS.md) for sourced genres/themes/demographics/studios and format/status/year/quarter, with dashboard apply/reset, retry and bounded result expansion.
- Tracker create/list/update/remove, aggregate or exact episode selections (including gaps), 1–5 ratings and a dashboard episode editor.
- Verified PostgreSQL private notes with conditional saves, completed-pass archives, explicit rewatches and guarded current-pass edits, with dashboard controls and account isolation.
- Verified PostgreSQL watch-list import with preview, export and atomic bulk statuses and ratings.
- Verified-owner followed schedule calendar with bounded date windows, track/season scope and current schedule revisions, explicitly labeled as unconfirmed.
- Individual-season or linked-franchise follows, release-track preferences and owner-scoped in-app episode/season-announcement notifications, polled from the dashboard with an unread filter/count, bulk read, bounded catch-up status and oldest pending release. The follows API offers opt-in pagination.
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
- Canonical catalog/provenance schema, 40-feature taxonomy, reviewed evidence import, verified-owner Personal Match and feature-filtered Discovery, dismissals and planning actions.
- CI for Go race tests, vet/build, PostgreSQL runtime/HTTP isolation, migration apply/rollback, frontend transport/session tests, mocked and real-PostgreSQL browser tests, and frontend typecheck/build; CodeQL and dependency review.

Public source freshness and latest schedule counts are available through `go run ./cmd/catalog-status`; see [catalog operations](docs/CATALOG_OPERATIONS.md). Selected CC0 facts can be fetched with `cmd/catalog-fetch` before reviewed import; this does not enable scheduled availability collection.

Operator queue counts, outcomes and age are available through `go run ./cmd/identity-status` (JSON) or `-format=prometheus` with a restricted database connection; see [queue operations](docs/IDENTITY_QUEUE_OPERATIONS.md) and [disabled monitoring examples](deployments/monitoring/README.md).

API probes: `/healthz` checks process liveness; `/readyz` checks live runtime dependencies with a bounded, generic response. Readiness does not approve public deployment. Request logs use route templates rather than raw URL paths.

## Run locally

Use Go 1.26 and Node.js 22 with npm. PostgreSQL 17 and its client tools are needed for database-backed development and integration tests.

### Single-user development

From the repository root, start the API in a shell without `DATABASE_URL` or identity-mail configuration.

```sh
go run ./cmd/api
```

In a second shell, start the dashboard.

```sh
cd web
npm ci
npm run dev
```

Open [the dashboard](http://localhost:3000). The API defaults to `127.0.0.1:8080`, writes to `./data/local.json`, and exposes `/healthz` and `/readyz`. Set `DATA_FILE` or `HTTP_ADDR` in the API environment to override local defaults. The browser defaults to `http://localhost:8080/api/v1`. Set `NEXT_PUBLIC_API_BASE_URL` before starting or building the dashboard to change it. The development CORS configuration allows browser origins `http://localhost:3000` and `http://127.0.0.1:3000`.

The [local Compose file](deployments/docker-compose.yml) is an alternative for JSON mode. Run `docker compose -f deployments/docker-compose.yml up --build`. The host ports bind to loopback. Compose waits for the API readiness probe before starting the dashboard. The dashboard API URL is set at image build time with `NEXT_PUBLIC_API_BASE_URL`. Both application images run as non-root. Run `scripts/check-containers.sh` for isolated image startup and health checks.

### PostgreSQL development

1. Create an empty local PostgreSQL 17 database. Provide its owner connection as `MIGRATION_DATABASE_URL` and run `go run ./cmd/migrate` from the repository root.
2. Apply `deployments/runtime-role.sql` to that database as its administrator. Set the runtime role's password outside Git and supply the restricted connection as `DATABASE_URL`.
3. Start `go run ./cmd/api` and the dashboard. A bad configured database connection or missing schema/grants stops PostgreSQL startup. Reapply the runtime grants after schema upgrades as directed by the migration notes.
4. To exercise verified-account features locally, follow the test mailbox or TLS SMTP setup in the [PostgreSQL runbook](docs/POSTGRES_RUNTIME.md). With mail unconfigured, catalog and account controls work, but newly registered accounts cannot complete verification for private product features.

Keep the migration owner, API role and optional catalog roles separate. The [PostgreSQL runbook](docs/POSTGRES_RUNTIME.md) covers roles, mail and runtime verification.

### Tests

From the repository root:

```sh
make check
# With TEST_DATABASE_URL set to a disposable test administrator connection:
go test -race -count=1 -timeout=3m -v ./internal/platform/postgres
```

The PostgreSQL suite skips without `TEST_DATABASE_URL`. It needs permission to create/drop isolated databases and roles. Use a dedicated development/test server and synthetic data.

From `web`:

```sh
npm ci
npm test
npm run typecheck
npm run build
npx playwright install chromium
npm run test:e2e
```

Playwright runs mocked API suites by default. On Linux CI, install Chromium system dependencies with `npx playwright install --with-deps chromium`. For real PostgreSQL and identity browser suites, follow the [runbook](docs/POSTGRES_RUNTIME.md#full-stack-browser-verification) and the environment in the [CI workflow](.github/workflows/ci.yml). CI also runs vulnerability/dependency checks, migration rollback, recovery, monitoring and CodeQL.

The [backup recovery guide](docs/BACKUP_RECOVERY.md) includes a synthetic logical-restore CI drill and an offline recovery fence. Restored historical credentials and account changes must be reconciled before reopening; no backup service or deployment is activated.

## Development boundary

**Do not deploy publicly or use sensitive data yet.** JSON mode is single-user and unauthenticated. PostgreSQL authentication and isolation have automated coverage but still need independent security review, production abuse controls, operational mail/key configuration, queue monitoring, and operational hardening. Browser cookie authentication is not implemented; the API accepts explicit bearer headers only. The bounded Wikidata fact and reviewed TVmaze season/schedule workers are opt-in and idle by default, with durable per-title scheduling, bounded retries and atomic import acknowledgements; see [worker lifecycle and upgrade](docs/CATALOG_WORKER.md) and [TVmaze data scope/attribution](docs/TVMAZE.md). Real source activation, broad licensed metadata/availability coverage, broader provider adapters, background delivery channels and hybrid AI remain pending. See [catalog ingestion](docs/CATALOG_FEEDS.md), [provider approval registry](docs/PROVIDERS.md), [episode progress](docs/EPISODE_PROGRESS.md) and [release alerts](docs/RELEASE_ALERTS.md).

Only source code, synthetic fixtures, and permitted catalog assets belong in this public repository. Never commit secrets, real account data, personal watch histories, or database dumps.
