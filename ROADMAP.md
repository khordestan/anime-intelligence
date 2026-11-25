# Anime Intelligence roadmap

Build one product for anime tracking and discovery. [PROGRESS.md](PROGRESS.md) describes the verified runtime and the next work. The [product specification](docs/PRODUCT_SPEC.md) defines the feature areas and the [source policy](docs/DATA_SOURCE_POLICY.md) governs catalog expansion.

## Product requirements

| Area | Target |
| --- | --- |
| Catalog | Comprehensive, current metadata with canonical titles, seasons, franchises, relations, staff, characters, studios, genres, subgenres, themes, demographics and nuanced styles. Preserve reviewed provenance and scope. Keep scheduled airings distinct from confirmed releases and track regional/language availability where licensed. |
| Library | Personal statuses, aggregate and exact per-episode progress, season and series ratings, notes, rewatches, bulk actions and portable data. |
| Release intelligence | Follow seasons or franchises and notify on newly announced seasons, episodes, movies and revised schedules or delays when explicitly confirmed by an approved source. Support delivery with the dashboard closed. |
| Taste and discovery | Separate declared and inferred taste signals with confidence and supporting evidence. Offer Personal Match, Global Essentials independently of personal fit, and Discovery Picks that broaden exploration. Learn from explicit feedback and evaluate retrieval and ranking. |
| Hybrid AI | Keep retrieval, permissions and deterministic ranking inside the application. Provide a natural-language analyst over typed tools with sourced explanations, spoiler controls, model provenance and usage controls. |
| Launch quality | Verified ownership, secure multi-user isolation, export and deletion, licensed sources, bounded and idempotent processing, resilience, observability, reproducible backups and recovery, and explicit authorization for public launch. |

## Partial capabilities

| Capability | Available | Remaining |
| --- | --- | --- |
| Catalog freshness and coverage | Reviewed imports, opt-in bounded jobs, exact mappings, review/retraction tools and metrics | Licensed comprehensive sources, broader people/character data, confirmed availability, active operations and quality objectives |
| Season-level library | Canonical title-level progress, ratings, relations and viewing passes | Finer season and rating conventions, franchise corrections and more bulk editing |
| Alerts | Verified-owner in-app episode and season-announcement notices, follow pagination and an upcoming schedule calendar | Delay/movie alert rules, explicit corrections, background channels and operational delivery |
| Recommendations | PostgreSQL Personal Match, offline evaluators, opt-in Global Essentials and paginated Discovery with evidence navigation and dismissals | Reviewed Essentials corpus, deliberate exploration beyond evidence coverage, representative judgments and feedback learning |
| Production operations | Security tests, recovery drill, bounded diagnostics and monitoring examples | Independent review, ingress abuse controls, load testing, activated monitoring, real backup/recovery and deployment workflows |

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
- [x] Confirmed-release catalog filter based on latest observed episode and track facts
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
- [x] Dashboard watch-list import and API preview
- [x] Atomic bulk series ratings API
- [x] Dashboard bulk series ratings
- [x] Atomic bulk one-episode progress for count-mode tracker entries
- [ ] Further bulk editing actions
- [x] Transitive linked-season follows with explicit per-title scope

## Milestone 4 — Release intelligence
- [x] Transactional offline schedule ingestion and revision history
- [x] Public episode schedule revision pages with per-track source attribution
- [x] Offline schedule-move preview across reviewed snapshot pairs
- [x] Opt-in bounded Wikidata factual synchronization with persisted source due time
- [x] Durable explicit-ID Wikidata jobs, per-title scheduling, bounded retries and fenced atomic import acknowledgement
- [ ] Broad approved metadata/availability synchronization and catalog discovery
- [x] Durable post-baseline season-announcement and confirmed episode-release detection
- [x] Owner-scoped polling in-app episode notifications and per-follow release tracks
- [x] Dashboard notification controls, unread filters/counts, bulk read and backlog catch-up timestamp
- [x] Keyset pagination for the verified follows API and dashboard, with per-title subscription lookup
- [x] Followed schedule calendar with latest revisions, track/scope filters and explicit unconfirmed labels
- [x] Verified-owner release email consent API
- [x] Opt-in background release email with a leased queue and bounded retries
- [x] Atomic release email metrics textfile and disabled collector service examples
- [x] Aggregate release email queue diagnostics in JSON and Prometheus text
- [x] Disabled release email scheduler freshness, overdue-scan and exhaustion alert rules with Prometheus tests
- [ ] Production release email activation and other delivery channels
- [x] Durable canonical release-event and owned notification duplicate suppression

## Milestone 5 — Taste intelligence
- [x] Owner-scoped declared preferences and bounded runtime rating-derived profile signals
- [x] Private library status and rating distribution summary
- [ ] Richer profile analytics and feedback learning
- [x] Explain inferred feature signals with contributing rating counts
- [ ] Broader explainable analytics
- [x] Deterministic Personal Match ranking core (pure Go, explainable, unit-tested)
- [x] Verified-owner PostgreSQL Personal Match API with conservative curated non-spoiler evidence
- [x] Preference editing and evidence-backed Personal Match dashboard
- [x] Candidate retrieval by profile overlap with eligible curated evidence
- [x] Offline retrieval-cap evaluation on held-out judgments
- [ ] Richer curated evidence coverage and representative retrieval evaluation
- [x] Evidence-backed cold-start Discovery lane with keyset pagination, curated feature labels/filtering and add to Planning
- [x] Offline Global Essentials ranking rubric and evaluator
- [x] Optional Global Essentials API and dashboard lane for reviewed manifests
- [x] Discovery ranking by curated features missing from the caller’s library
- [ ] Reviewed Global Essentials corpus and representative Discovery ranking evaluation
- [x] Confirmed in-app movie release notices for eligible follows
- [x] Opt-in in-app notices for reviewed earlier and later schedule moves
- [x] In-app title cancellations backed by explicit reviewed primary evidence
- [ ] Explicit authoritative delay notices and cancellation corrections
- [ ] Recommendation and alert dashboards for finer controls where the API already supports them
- [x] Offline Personal Match ranking evaluator with repeatable fixtures and a baseline
- [ ] Feedback learning and representative evaluation datasets

## Milestone 6 — AI and discovery
- [x] Read-only catalog evidence tool with bounded topic selection and source attribution
- [x] Opt-in model-assisted catalog topic selection
- [ ] Tool-constrained analyst for broader queries
- [x] Two-title catalog comparison with sourced numeric explanations
- [ ] Evidence-grounded explanations across broader analyst tools
- [x] Owner-scoped Personal Match dismissals and dashboard undo
- [x] Discovery hide/restore and add-to-Planning feedback
- [x] Explicit feature feedback from Discovery through the private Personal Match preference controls
- [ ] Discovery feedback beyond hide/restore and library actions
- [x] LLM provider abstraction, OpenAI adapter and process-local usage controls
- [x] Shared PostgreSQL model budgets across API replicas
- [ ] Representative model evaluation and deployment quota tuning

## Milestone 7 — Production readiness
- [x] Request-log redaction and bounded live dependency/security-schema readiness checks
- [x] Loopback-tested ingress source limits for credential and model endpoints
- [ ] Production ingress configuration, independent security review and broader observability
- [x] Synthetic PostgreSQL logical restore/key recovery and offline credential-fencing CI drill
- [x] Isolated PostgreSQL physical base backup and archived WAL restore to a named recovery point
- [ ] Real backup/key storage, post-snapshot deletion/security reconciliation, PITR and recovery objectives
- [x] Synthetic PostgreSQL release backlog bursts, isolation and overlapping franchise-path performance test
- [x] Concurrent synthetic HTTP catalog and private library workload with atomic progress retry checks
- [ ] Representative load and reliability tests
- [x] Browser auth/account-switch/local-mode tests and transport race tests
- [x] Full-stack browser tests against the real PostgreSQL API (dedicated disposable CI database and restricted runtime role)
- [x] Non-root application container builds and isolated startup smoke checks
- [ ] Production deployment automation and operational runbooks
