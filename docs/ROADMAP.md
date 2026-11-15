# Delivery Roadmap

## Milestone 0 — Runnable foundation
- [x] Go API process and graceful shutdown
- [x] Local persistent development store
- [x] Catalog search and detail endpoint
- [x] Tracker create/list/update, episode progress and ratings
- [x] Next.js dashboard
- [x] Go tests, race tests, vet/build and frontend CI
- [x] PostgreSQL canonical schema draft
- [x] Provider-neutral starter catalog seed
- [x] Curated 40-feature product taxonomy
- [x] Authentication credential/session schema foundation (migration 000004; no runtime auth)
- [x] Live PostgreSQL migration execution and runtime repository
- [x] Authentication endpoints and secure bearer-session lifecycle (production security review pending)
- [x] Enforced runtime authorization and tested user isolation

## Milestone 1 — Identity and persistence
- [x] Authentication/session data model and constraints
- [x] PostgreSQL repository interfaces and implementation (catalog, tracker, identity)
- [x] Versioned migration runner and runtime integration tests
- [x] Argon2id password hashing and verification
- [x] Registration, login, logout, session refresh/revocation
- [x] Authorization and user-scoped queries
- [x] PostgreSQL RLS policies and transaction-scoped user context contract (migration 000005; PostgreSQL runtime context implemented)
- [x] Browser authentication UI with in-memory tokens and account-switch isolation
- [x] Session listing and remote session revocation (PostgreSQL API)
- [ ] Session-management dashboard controls
- [ ] Password recovery, email verification and production abuse controls
- [x] User data export and password-confirmed deletion (PostgreSQL API)
- [ ] Account export/deletion dashboard controls

## Milestone 2 — Catalog and data platform
- [x] Initial feature taxonomy covering story, protagonist, powers, tone, pacing and spoiler-sensitive endings
- [ ] Provider capability registry and license review
- [ ] Provider adapters and external ID mapping
- [ ] Raw source snapshots where licensing permits
- [ ] Canonical entity resolution and field-level provenance
- [ ] Curated observations with evidence, confidence and spoiler scope
- [ ] Data quality and freshness monitoring
- [ ] Franchise, season and episode relation graph

## Milestone 3 — Tracker completeness
- [ ] Season and episode-level progress
- [ ] Notes, rewatches and season ratings
- [ ] Import/export and bulk editing
- [ ] Franchise follows

## Milestone 4 — Release intelligence
- [ ] Schedule synchronization and revision history
- [ ] Announcement and release event detection
- [ ] In-app notifications and notification preferences
- [ ] Idempotency and duplicate suppression

## Milestone 5 — Taste intelligence
- [ ] Declared and inferred preference profiles
- [ ] Explainable analytics
- [x] Deterministic Personal Match ranking core (pure Go, explainable, unit-tested)
- [ ] PostgreSQL-backed recommendation API
- [ ] Personal Match, Global Essentials and Discovery lanes
- [ ] Feedback learning and offline evaluation

## Milestone 6 — AI and discovery
- [ ] Tool-constrained AI analyst
- [ ] Evidence-grounded explanations
- [ ] Discovery feedback experience
- [ ] LLM provider abstraction and usage controls

## Milestone 7 — Production readiness
- [ ] Security review, rate limiting and observability
- [ ] Backups, load and reliability tests
- [x] Browser auth/account-switch/local-mode tests and transport race tests
- [ ] Full-stack browser tests against the real PostgreSQL API
- [ ] Deployment automation and operational runbooks
