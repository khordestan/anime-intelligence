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
- [x] Session-management dashboard controls
- [x] Authenticated password changes with current-password verification and revocation of all existing sessions
- [x] Hashed, purpose-bound password-recovery/email-verification challenge model and redemption
- [x] Recovery/verification delivery, API and dashboard flows with verified-TLS SMTP and mail-enabled browser CI
- [x] Mandatory verified-email policy for PostgreSQL tracker and ratings, with account/recovery access preserved
- [x] Bounded volatile identity-delivery retries with proof-expiry/shutdown handling
- [x] Encrypted durable delivery of issued identity challenges, leased claims and persisted retries
- [x] Encrypted durable request acceptance before challenge issuance, shared queue capacity and atomic consumption
- [x] Bounded inbox/outbox decryption keyring and planned rolling-rotation runbook
- [x] Read-only queue diagnostics command without private data with consistent aggregate snapshots
- [x] Allowlisted queue terminal outcomes with conservative legacy classification
- [x] Bounded Prometheus text export from the queue diagnostic CLI
- [x] Atomic collector snapshots, disabled systemd/Prometheus examples and promtool-tested alert rules
- [ ] Host activation/target inventory/receiver routing and broader provider telemetry, automated secret rollout and key/backup recovery exercises
- [x] Persisted per-account password-failure window shared across login, password changes, deletion and API instances
- [ ] Production ingress/distributed abuse controls and monitoring
- [x] User data export and password-confirmed deletion (PostgreSQL API)
- [x] Account export/deletion dashboard controls

## Milestone 2 — Catalog and data platform
- [x] Initial feature taxonomy covering story, protagonist, powers, tone, pacing and spoiler-sensitive endings
- [x] Provider capability/approval registry and initial primary-source review
- [ ] Production source-use permissions and licensed network activation
- [x] Offline reviewed snapshot importer with exact external ID mapping
- [x] Bounded explicit-item Wikidata CC0 factual fetch adapter
- [x] Bounded reviewed TVmaze season metadata/schedule adapter with attribution and durable source-isolated jobs
- [ ] Comprehensive approved metadata and confirmed-availability adapters
- [x] Opt-in selected neutral metadata observations, full transactional import preview and bounded operator field-change history
- [ ] Raw source snapshots where licensing permits
- [x] Reviewed exact provider identity links with audit/replay protection and public lookup/display
- [x] Explicit reviewed secondary episode-source imports with per-title baselines and preserved metadata authority
- [x] Reviewed label-field choices with retained primary observations, source provenance, restoration and refresh-safe projections
- [x] Reviewed release-year/quarter/format/episode-total choices with atomic projection and retained-episode conflict checks
- [x] Reviewed title/native-name/synopsis/display-status choices with primary-only announcement baselines
- [x] Reviewed unused provider-link retraction with dependency checks, immutable audit and import resurrection fence
- [ ] Canonical entity merges and dependent-data correction/retraction
- [x] Reviewed offline observation ingestion with evidence/confidence and conservative non-spoiler series scope
- [ ] Real evidence corpus and broader reviewed scope coverage
- [x] Read-only source/attempt/per-title freshness and metadata-presence diagnostics
- [x] Aggregate catalog freshness/metadata-presence Prometheus diagnostics with parser CI
- [ ] Catalog SLOs/alert activation, field accuracy and completeness evaluation
- [x] Validated canonical prequel/sequel graph and named bidirectional direct continuity navigation
- [x] Bounded public transitive prequel/sequel API and dashboard browsing with source labels and cap warnings
- [x] Combined sourced-category/format/status/year/quarter catalog filters with explicit dashboard controls and bounded result expansion
- [x] Bounded available-value facets and explicit dashboard label discovery
- [x] Offset navigation hints and dashboard filtered paging with retry
- [x] Stable cursor pagination for the public catalog API
- [x] Dashboard cursor navigation with destination retry
- [ ] Broader catalog browsing
- [ ] Reviewed franchise curation/retractions and finer season conventions

## Milestone 3 — Tracker completeness
- [x] Transactional owner-scoped tracker removal API with ratings/progress cleanup
- [x] Dashboard removal confirmation, retry/cancel and browser isolation coverage
- [x] Exact episode selections per canonical season/title, separate from aggregate count mode
- [x] Dashboard exact-episode editing
- [x] Completed-pass archives, conditional rewatch starts and dashboard history
- [x] Library title search, status/rating filters and explicit title/rating/episode ordering
- [ ] Finer season conventions
- [x] Private tracker notes with conflict detection and dashboard editing
- [ ] Finer season rating conventions
- [x] Atomic, viewing-pass-guarded bulk status edit API
- [x] Dashboard bulk status editing with stale-entry recovery
- [x] Versioned current watch-list export and dashboard download
- [x] Atomic, add-only watch-list import API
- [x] Dashboard watch-list import
- [x] Atomic bulk series ratings API
- [x] Dashboard bulk series ratings
- [ ] Further bulk editing actions
- [x] Transitive linked-season follows with explicit per-title scope

## Milestone 4 — Release intelligence
- [x] Transactional offline schedule ingestion and revision history
- [x] Opt-in bounded Wikidata factual synchronization with persisted source due time
- [x] Durable explicit-ID Wikidata jobs, per-title scheduling, bounded retries and fenced atomic import acknowledgement
- [ ] Broad approved metadata/availability synchronization and catalog discovery
- [x] Durable post-baseline season-announcement and confirmed episode-release detection
- [x] Owner-scoped polling in-app episode notifications and per-follow release tracks
- [x] Dashboard notification controls
- [x] Followed schedule calendar with latest revisions, track/scope filters and explicit unconfirmed labels
- [ ] Background delivery channels
- [x] Durable canonical release-event and owned notification duplicate suppression

## Milestone 5 — Taste intelligence
- [x] Owner-scoped declared preferences and bounded runtime rating-derived profile signals
- [ ] Richer profile analytics and feedback learning
- [x] Explain inferred feature signals with contributing rating counts
- [ ] Broader explainable analytics
- [x] Deterministic Personal Match ranking core (pure Go, explainable, unit-tested)
- [x] Verified-owner PostgreSQL Personal Match API with conservative curated non-spoiler evidence
- [x] Preference editing and evidence-backed Personal Match dashboard
- [x] Candidate retrieval by profile overlap with eligible curated evidence
- [x] Offline retrieval-cap evaluation on held-out judgments
- [ ] Richer curated evidence coverage and representative retrieval evaluation
- [ ] Personal Match, Global Essentials and Discovery lanes
- [x] Offline Personal Match ranking evaluator with repeatable fixtures and a baseline
- [ ] Feedback learning and representative evaluation datasets

## Milestone 6 — AI and discovery
- [ ] Tool-constrained AI analyst
- [ ] Evidence-grounded explanations
- [x] Owner-scoped Personal Match dismissals and dashboard undo
- [ ] Discovery feedback experience
- [ ] LLM provider abstraction and usage controls

## Milestone 7 — Production readiness
- [x] Request-log redaction and bounded live dependency/security-schema readiness checks
- [ ] Security review, rate limiting and broader observability
- [x] Synthetic PostgreSQL logical restore/key recovery and offline credential-fencing CI drill
- [ ] Real backup/key storage, post-snapshot deletion/security reconciliation, PITR and recovery objectives
- [ ] Load and reliability tests
- [x] Browser auth/account-switch/local-mode tests and transport race tests
- [x] Full-stack browser tests against the real PostgreSQL API (dedicated disposable CI database and restricted runtime role)
- [ ] Deployment automation and operational runbooks
