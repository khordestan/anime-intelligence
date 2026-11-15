# Implementation Log

## 2026-11-15 — PostgreSQL row-level security foundation

Implemented:
- Added migration `000005_row_level_security` with forced RLS policies for users, auth sessions, watch history, ratings, progress, preferences, recommendations, recommendation feedback, follows, and notifications.
- Added a transaction-scoped `app_current_user_id()` helper reading the trusted `app.user_id` setting.
- Added PostgreSQL CI assertions for RLS enablement/force and policy count, plus an application-role isolation test proving one user cannot see another user's account row.
- Updated security and architecture documentation to clarify that RLS schema policies exist but the API does not yet establish a trusted per-request database identity.

Security boundary:
- RLS is not a replacement for authentication. The API still uses the local JSON store and does not set `app.user_id`.
- The setting must only be populated by trusted server code after session verification, inside the same transaction as protected queries. Never accept it from client input.
- The application role must be non-superuser, must not have `BYPASSRLS`, and must not own protected tables.

Verification:
- CI has been extended to apply and roll back migration 000005 and exercise RLS using a restricted database role. The new run has not yet completed.

## 2026-11-15 — Identity schema foundation

Implemented:
- Added migration `000004_auth_sessions`.
- Required account email and added case-insensitive uniqueness.
- Added password-hash storage, email verification, account disablement, failed-login and lockout metadata.
- Added opaque auth-session records with unique 32-byte token digests, expiry, revocation, and cleanup indexes.
- Added CI checks for migration application/rollback, case-insensitive email uniqueness, and rejection of invalid session token digest lengths.
- Added `docs/IDENTITY_SECURITY.md` describing password hashing, session handling, CSRF, rate limiting, authorization, and tenant isolation requirements.
- Updated README, architecture, and roadmap to distinguish schema work from runtime authentication.

Security boundary:
- No login or registration endpoint exists yet.
- No password hashing or session issuance code exists yet.
- The API still uses the single-user local JSON store; it must not be exposed publicly.
- PostgreSQL RLS is not enabled yet because the runtime does not establish a trusted per-request database identity.

Verification:
- GitHub Actions now runs PostgreSQL 17 migration and rollback checks including migration 000004 and auth constraint assertions. The latest workflow result is pending and must be checked before calling it passed.

## 2026-11-15 — Public repository security baseline

Implemented:
- Added `SECURITY.md` with private vulnerability-reporting guidance and explicit development-stage security boundaries.
- Added `CONTRIBUTING.md` with test expectations, data provenance requirements, and a strict prohibition on committing personal watch histories or secrets.
- Added Dependabot update proposals for Go modules, npm dependencies, and GitHub Actions.
- Added CodeQL workflows for Go and JavaScript/TypeScript plus dependency review on pull requests.
- Updated the README to state the public-repository privacy rules and current non-production limitations.

Verification:
- GitHub accepted commits creating all four security/contribution files.
- Workflow execution is pending; no CodeQL or dependency-review result is claimed yet.

## 2026-11-15 — Foundation and intelligence core

Implemented:
- Go API process with health, catalog search/detail, tracker create/list/update, progress, and rating routes.
- Local JSON-backed development store and Next.js dashboard.
- Request validation, bounded request bodies, local-origin CORS, structured request logging, graceful shutdown.
- PostgreSQL canonical schema for users, anime, source records, feature observations, user tracking/ratings/preferences, recommendations, airing schedules, domain events, follows, and notifications.
- Provider-neutral starter catalog fixtures.
- Curated 40-key feature taxonomy, including spoiler-sensitive ending/revelation features.
- Deterministic Personal Match ranking core: explicit and inferred preference separation, confidence-weighted profile inference, explainable evidence, candidate exclusion, deduplication, bounded score and algorithm version.
- GitHub Actions CI for Go tests, race detector, vet/build, frontend typecheck/build, and PostgreSQL migration/rollback validation.

Verified:
- Local Go: `go test -race ./...`, `go vet ./...`, and `go build ./...` passed for the local vertical slice.
- GitHub Actions: backend tests, race detector, vet/build and frontend typecheck/production build passed on commit `0fb6789`.
- GitHub Actions PostgreSQL 17 service: all three up migrations, assertions for 8 anime and 40 taxonomy features, and all three down migrations passed on the CI run that includes the database job.

Not yet implemented:
- PostgreSQL runtime repository and API wiring. The running API still uses local JSON persistence.
- Authentication, sessions, authorization, and enforced multi-user isolation.
- Loading catalog observations into the recommendation engine or exposing recommendations through HTTP.
- Licensed external data providers, AI analyst, discovery UI, and release notifications.

Environment limitations:
- Local environment cannot reach the Go module proxy or run a PostgreSQL server. The PostgreSQL CI service is being used to validate migrations.
- The Next.js production build is validated in GitHub Actions rather than this local environment.

## 2026-11-15 — PostgreSQL runtime, identity and tenant isolation

Implemented:
- Added a request-aware repository interface and PostgreSQL implementations for catalog, tracker, progress, and ratings while preserving local JSON development mode.
- Wired optional `DATABASE_URL` startup, restricted runtime-role checks, forced-RLS verification, and transaction-local identity from verified session digests. No migration-owner fallback.
- Added Argon2id hashing with fixed resource bounds; registration, login, current account, logout, and atomic refresh endpoints; 256-bit random opaque bearer tokens stored only as SHA-256 digests with 24-hour expiry.
- Added explicit user predicates and private-query session revalidation/locking, generic login failures, bounded authentication bodies, process-wide rate/concurrency limits, and no-store private responses.
- Added the embedded transactional migration runner, checksum/history validation, advisory locking, and separate role-provisioning documentation.
- Updated Go/dependency versions, OpenAPI, runtime/security documentation, roadmap, and local-only Compose port bindings.
- Extended PostgreSQL CI to run race-enabled runtime/HTTP integration tests using disposable databases and restricted roles.

Verified locally:
- PostgreSQL 17.12 integration tests and `go test -race -count=1 ./...` pass, including cross-user reads/writes, refresh races, rollback/context cleanup, disabled/expired/revoked tokens, repeated rating upserts, restricted-role startup, and migration integrity.
- `go vet ./...`, `go build ./...`, and `git diff --check` pass.

Remaining boundaries:
- No public deployment was performed. Independent security review and launch hardening remain required.
- Browser authentication UI, password recovery/email verification, session management, account export/deletion, and expired-session cleanup remain pending.
- GitHub CI results must be checked after this commit is pushed.
