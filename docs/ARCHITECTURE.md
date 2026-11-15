# Architecture

The first release is a modular monolith. The API and worker are separate processes that share domain packages. Avoid microservices until scale or deployment boundaries justify them.

## Current runtime
- HTTP handlers in `internal/modules` implement catalog and tracker use cases.
- `internal/platform/store.Repository` is the request-aware persistence boundary; a local JSON adapter is available for single-user development.
- `internal/platform/postgres` implements catalog, tracker, credentials, and session persistence. Every private repository operation resolves a verified bearer token and runs inside a transaction with local RLS context.
- Next.js consumes the versioned REST API.
- The worker is a lifecycle placeholder; no background jobs are registered. Identity mail runs in the API through an encrypted PostgreSQL request inbox (committed before HTTP 202) and an encrypted mail outbox, with leased issuance/delivery workers.
- PostgreSQL migrations through `000010_identity_queue_outcomes` define the canonical data model, auth/session primitives, and forced row-level security policies. The API selects PostgreSQL when `DATABASE_URL` is configured and establishes transaction-scoped identity from server-verified session digests.

## Target architecture
- Go REST API with explicit modules: identity, catalog, tracker, preferences, recommendations, notifications.
- PostgreSQL as the system of record; parameterized pgx queries and embedded checksummed migrations currently implement the persistence boundary. Reconsider sqlc when the query surface grows.
- Redis only when caching, rate limiting, or durable job coordination is justified.
- Provider adapters normalize external data into canonical entities with provenance.
- LLMs receive constrained application tools and evidence; they never query the database directly.

## Identity and security boundary
- Migrations `000004_auth_sessions` and `000005_row_level_security` add case-insensitive email uniqueness, password-hash storage, account security metadata, hashed session-token records, and forced per-user RLS policies.
- The intended password format is Argon2id PHC; the database stores only an encoded hash, never plaintext.
- Raw random session tokens are returned once and only SHA-256 digests are persisted.
- Account export uses a repeatable-read snapshot; password-confirmed deletion uses an exclusive account lock and foreign-key cascades. All session-backed transactions acquire account state locks before session locks.
- PostgreSQL mode implements login, registration, token rotation/revocation, authorization, and a bounded process-wide password gate. Cookie authentication is unsupported; explicit bearer headers avoid ambient-cookie CSRF. Production abuse controls and independent review remain pending.
- User identity must come from a verified server-side session, never a client-supplied user ID.
- User-owned data must be scoped by authenticated user ID in every query. Migration `000005_row_level_security` installs forced RLS policies using `app.user_id`; the PostgreSQL runtime sets this trusted context within each session-verified private transaction. RLS is defense in depth, not a substitute for correct application authorization.

See [IDENTITY_SECURITY.md](IDENTITY_SECURITY.md) for the application security contract.

## Important boundary
The current JSON store is single-process and single-user. It is not safe for public deployment or multiple API replicas. PostgreSQL mode is covered by authentication/isolation tests, but production release still requires security hardening and review.
