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
- [ ] Live PostgreSQL migration execution and runtime repository
- [ ] Authentication and enforced user isolation

## Milestone 1 — Identity and persistence
- [ ] PostgreSQL repository interfaces and implementation
- [ ] Versioned migration runner and integration tests
- [ ] Password/session or external identity authentication
- [ ] Authorization and user-scoped queries
- [ ] User data export and deletion

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
- [ ] Deterministic recommendation ranking
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
- [ ] Browser/end-to-end tests
- [ ] Deployment automation and operational runbooks
