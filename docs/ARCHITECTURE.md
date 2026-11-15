# Architecture

The first release is a modular monolith. The API and worker are separate processes that share domain packages. Avoid microservices until scale or deployment boundaries justify them.

## Current runtime
- HTTP handlers in `internal/modules` implement catalog and tracker use cases.
- `internal/platform/store` is a local JSON persistence adapter used for development.
- Next.js consumes the versioned REST API.
- The worker is a lifecycle placeholder; no background jobs are registered.

## Target architecture
- Go REST API with explicit modules: identity, catalog, tracker, preferences, recommendations, notifications.
- PostgreSQL as the system of record; sqlc-generated query layer and golang-migrate.
- Redis only when caching, rate limiting, or durable job coordination is justified.
- Provider adapters normalize external data into canonical entities with provenance.
- LLMs receive constrained application tools and evidence; they never query the database directly.

## Important boundary
The current JSON store is single-process and single-user. It is not safe for public deployment or multiple API replicas. Authentication and user-scoped authorization must precede multi-user release.
