# Architecture

The first release is a modular monolith. The API and worker are separate processes that share domain packages. Avoid microservices until scale or deployment boundaries justify them.

## Current runtime
- HTTP handlers in `internal/modules` implement catalog and tracker use cases.
- `internal/platform/store` is a local JSON persistence adapter used for development.
- Next.js consumes the versioned REST API.
- The worker is a lifecycle placeholder; no background jobs are registered.
- PostgreSQL migrations through `000004_auth_sessions` define the canonical data model and auth/session storage primitives. The API does not connect to PostgreSQL yet.

## Target architecture
- Go REST API with explicit modules: identity, catalog, tracker, preferences, recommendations, notifications.
- PostgreSQL as the system of record; sqlc-generated query layer and golang-migrate.
- Redis only when caching, rate limiting, or durable job coordination is justified.
- Provider adapters normalize external data into canonical entities with provenance.
- LLMs receive constrained application tools and evidence; they never query the database directly.

## Identity and security boundary
- Migration `000004_auth_sessions` adds case-insensitive email uniqueness, password-hash storage, account security metadata, and hashed session-token records.
- The intended password format is Argon2id PHC; the database stores only an encoded hash, never plaintext.
- Raw random session tokens are returned once and only SHA-256 digests are persisted.
- The schema is not an authentication implementation. Login, registration, token issuance, revocation, CSRF protection, rate limiting, and authorization remain unimplemented.
- User identity must come from a verified server-side session, never a client-supplied user ID.
- User-owned data must be scoped by authenticated user ID in every query. PostgreSQL RLS is planned as defense in depth, not a substitute for correct application authorization.

See [IDENTITY_SECURITY.md](IDENTITY_SECURITY.md) for the application security contract.

## Important boundary
The current JSON store is single-process and single-user. It is not safe for public deployment or multiple API replicas. Authentication and user-scoped authorization must precede multi-user release.
