# Implementation Log


## 2026-11-25 — Confirmed-release catalog filter

- Added an optional confirmed-release filter to catalog browsing. It uses the latest episode and track facts and preserves existing cursors when the filter is off.
- Focused PostgreSQL race and Playwright catalog filter tests passed.
## 2026-11-24 — Discovery evidence labels

Added bounded curated feature names to each Discovery title and documented the existing cursor fields in OpenAPI. Extended the PostgreSQL regression. Hosted CI is pending.

## 2026-11-23 — Discovery pagination

Added a cursor for Discovery and a dashboard load-more action. Extended PostgreSQL coverage for stable pagination across tied evidence counts. Hosted CI is pending.

## 2026-11-23 — Plan to watch from recommendations

Added the dashboard action using the existing add-only import and refreshed the library after a successful save. Fixed the Discovery load signature reported by TypeScript CI. Added a browser regression. Hosted CI is pending.

## 2026-11-22 — Cold-start Discovery

Added verified-owner discovery from curated evidence without a taste profile. Discovery respects tracker and dismissal exclusions and exposes counts rather than fit scores. Added PostgreSQL and browser regressions. Hosted CI is pending.

## 2026-11-22 — Manage saved Personal Match dismissals

Added cursor-based owner-scoped dismissal listing and a dashboard restore action. Added PostgreSQL and mocked browser regressions. Hosted CI is pending.

## 2026-11-22 — Personal Match dismissals

Added reversible owner-scoped title dismissals with forced RLS, account export, and dashboard hide/undo. Extended PostgreSQL tests for owner isolation and restoration. Hosted CI is pending.

## 2026-11-22 — Compact release-alert polling

Added a verified-owner notification summary endpoint for collapsed panels. It reuses bounded materialization and does not select notification text. Added PostgreSQL authorization and summary coverage and updated the browser regression. Hosted CI is pending.

## 2026-11-22 — Release badges with the panel collapsed

Extended visible-dashboard polling to display an unread badge when the alerts panel is collapsed. Added a clock-controlled browser regression. Hosted CI is pending.

## 2026-11-22 — Total unread release alerts

Added a snapshot-consistent owner-wide unread count to notification pages and displayed it in the dashboard. Extended PostgreSQL and mocked browser tests. Hosted CI is pending.

## 2026-11-22 — Recommendation retrieval evaluation

Added opt-in evaluation of the evidence-overlap stage and candidate cap, with per-case retrieval counts and macro recall. Added a regression that exercises cap loss, neutral evidence and disabled retrieval mode. Hosted CI is pending.

## 2026-11-22 — Ranking quality and result fill metrics

Extended the offline evaluator with binary nDCG and top-k fill rate. Regression coverage includes a relevant hit at rank two and a no-evidence ranking. Hosted CI is pending.

## 2026-11-22 — Candidate budget excludes neutral signals

Derived retrieval keys from the same default preference weighting as the scorer. Added Go and PostgreSQL regressions covering neutral-only candidates competing for the bounded candidate pool. Hosted CI is pending.

## 2026-11-22 — Offline Personal Match evaluation

Added a versioned, bounded JSON evaluator with recall and reciprocal rank against a deterministic title baseline. Added a synthetic regression for held-out judgments and rated-title exclusion. Hosted CI is pending.

## 2026-11-22 — Unread release alerts

Added server-side unread filtering before cursor pagination and a dashboard toggle. Extended PostgreSQL ownership/history and mocked browser tests. Hosted CI is pending.

## 2026-11-22 — Mark all release alerts read

Added a verified-owner bulk read action for existing in-app release notices and exposed it in the dashboard. Extended PostgreSQL tests for authorization, history, isolation and idempotency. Hosted CI is pending.

## 2026-11-22 — Rating support for inferred taste signals

Added per-feature contributing rating counts in the owner-scoped recommendation profile and the dashboard. Extended the PostgreSQL inference regression. Hosted CI is pending.

## 2026-11-22 — Evidence-based Personal Match candidate retrieval

Retrieve candidates with eligible evidence overlapping the caller's taste profile before applying the candidate cap. Extended the PostgreSQL regression with 510 alphabetically earlier titles without evidence. Hosted CI is pending.

## 2026-11-22 — Atomic bulk series ratings

Added a verified-owner bulk series ratings API using the same bounded batch transaction and viewing-pass guards as bulk status. Extended PostgreSQL coverage for isolation, rollback, persistence and preserved progress. The dashboard uses the existing library selection and adds a separate series rating action. Extended the browser regression to verify its request. All ten checks passed on [PR #21](https://github.com/khordestan/anime-intelligence/pull/21), including the PostgreSQL bulk-edit regression.

## 2026-11-22 — Watch-list import preview API

Added a preview of estimated additions and skips, sharing validation with import. Extended the PostgreSQL regression to verify counts, rejected unknown IDs and absence of tracker writes. Hosted CI is pending.

## 2026-11-22 — Watch-list import in the dashboard

Added a JSON file picker, explicit import action, success counts and catalog-mismatch feedback. Account changes reset the selected file and fence upload results. Added a mocked browser regression. Hosted CI is pending.

## 2026-11-22 — Watch-list import API

Added add-only import of the versioned watch-list format with verified ownership, atomic catalog validation and idempotent replays. Added a PostgreSQL regression for authorization, rollback, progress and ratings. Hosted CI is pending.

## 2026-11-18 — Versioned watch-list export

Added a current-state watch-list JSON attachment and dashboard download. Added a disposable PostgreSQL regression for owner scope, verification and exact versus aggregate episode progress. Documented cursor pagination, bulk status and watch-list export in OpenAPI. Local Go and Node execution are unavailable. The [CI](https://github.com/khordestan/anime-intelligence/actions/runs/44300158006) and [security](https://github.com/khordestan/anime-intelligence/actions/runs/44300158012) workflows passed for export commit `2870a05`.

## 2026-11-18 — Dashboard bulk status editing

Added multi-entry selection and status editing for verified PostgreSQL accounts. The Playwright regression covers stale-pass recovery and successful retry with refreshed viewing passes. Local Node execution is unavailable. All seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44299413773) and all three [security jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44299413711) passed for dashboard commit `cf7c94d`, including all 37 mocked browser tests.

## 2026-11-18 — Atomic bulk tracker status edits

Added a verified-owner PostgreSQL API for status-only edits across up to 25 entries. Reused single-entry update logic while holding one product transaction. Added an HTTP/PostgreSQL regression for verification, isolation, duplicates, stale-pass rollback and exact-progress preservation. Local Go execution is unavailable. Hosted CI is pending.

## 2026-11-18 — Unfiltered dashboard catalog browsing

Added explicit Browse catalog mode with the existing cursor navigation, loading and retry controls. The Playwright regression exercises opening and closing browsing with no query or filter. Local Node execution is unavailable. All seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44298014862) and all three [security jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44298015076) passed for browse commit `fd4511f`.

## 2026-11-18 — Dashboard catalog cursor navigation

Updated filtered catalog searches, next/previous navigation and destination retry to use API cursors. Updated the synthetic Playwright paging regression. Local Node execution is unavailable. All seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44297477106) and all three [security jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44297477190) passed for dashboard navigation commit `8464d04`, including all 36 mocked browser tests.

## 2026-11-18 — Catalog API keyset pagination

Added opt-in catalog cursors with normalized filter binding, exact successor detection and unchanged offset compatibility. Added a hosted PostgreSQL paging regression. Local Go execution is unavailable in this workspace. All seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44297002555) and all three [security jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44297002557) passed for API commit `b857c8e`, including the PostgreSQL traversal regression.

## 2026-11-18 — Retired provider IDs in the catalog queue

The queue skips reviewed retired provider IDs before inserting or claiming jobs. Admission holds the source row lock also used during retirement, so an existing job blocks retirement and a committed retirement blocks subsequent admission. Added a disposable PostgreSQL regression for retired-only and mixed configurations using a synthetic Wikidata fetcher. Local tests were unavailable in this session. For implementation commit `cde7afb`, all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44291409852) passed, including the new PostgreSQL regression. Both CodeQL jobs passed. [Dependency review](https://github.com/khordestan/anime-intelligence/actions/runs/44291409772/job/134757272964) could not run because GitHub reports the repository dependency graph is unavailable.

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

## 2026-11-15 — Dependency security and reproducible frontend builds

- The PostgreSQL runtime commit passed GitHub backend, frontend, database runtime, migration/rollback, and CodeQL jobs (CI run 43662386866, Security run 43662386880).
- Local npm audit found critical/high advisories in the previous Next.js 15.5.6 dependency tree. Testing the latest 15.5 patch still left vulnerable transitive dependencies, so the small starter app was upgraded to Next.js 16.4.6 and React/React DOM 19.1.10.
- Added the npm lockfile, `npm ci`, an npm high-severity audit gate, and a pinned Go govulncheck CI invocation. Generated Next.js/TypeScript state is ignored.
- Local `npm audit --audit-level=low` reports zero vulnerabilities; frontend typecheck and Next.js 16 production build pass. Go govulncheck on Go 1.26.11 reports no vulnerabilities.
- Public deployment remains disabled. No live credentials or account data were added.

## 2026-11-15 — Browser authentication and account-switch isolation

- Added runtime capability metadata and PostgreSQL login/registration, logout, expiry, and unauthorized-session handling in the dashboard, preserving unauthenticated JSON development mode.
- Browser bearer tokens live only in component-scoped memory. Requests omit cookies and caches. Account changes clear private UI state and abort/discard outstanding responses, including responses still decoding JSON.
- Added transport race tests and Playwright browser coverage for registration/login, private library mutation, account switching, reload credential loss, rejected-session cleanup, and local-mode compatibility.
- Local transport tests, frontend typecheck, and production build pass. The workspace prohibits browser Unix sockets, so Chromium cannot launch locally; browser tests are added to GitHub CI with retained test results for verification there.
- The dependency/security commit passed GitHub CI and CodeQL (CI run 43662636760, Security run 43662636718). No public deployment was performed.


## 2026-11-15 — Browser fetch regression and failure-path coverage

- The first browser CI run exposed native `fetch` being invoked with the API client as its receiver, causing an illegal-invocation error in the browser. Fixed by explicitly preserving the global receiver and added a native-style transport regression test.
- Browser CI now tests the built production server. Docker frontend builds also use the committed lockfile via `npm ci`.
- Added locally passing PostgreSQL race tests for atomic migration failure rollback and denial of locked/unprovisioned accounts. Backend and database jobs passed in the previous CI run; browser verification is rerun after this fix.

## 2026-11-15 — Account data portability and deletion

- Added session-protected product-data export using a consistent repeatable-read snapshot, explicit owner predicates, forced RLS, stable row ordering, and bounded serialization retries. Password/session hashes and security metadata are excluded.
- Added password-confirmed account deletion with the hashing abuse gate, exclusive account locking, atomic foreign-key cascades, and invalidation of all account sessions. Request-provided target user IDs are rejected.
- Standardized account-before-session lock ordering and tested concurrent deletion from separate sessions. Runtime startup now checks all ten private tables and rejects role/database creation and replication privileges in addition to owner/superuser/BYPASSRLS roles.
- Extended least-privilege runtime grants and made provisioning safe to rerun. Updated OpenAPI, roadmap, security contract, and runtime documentation. Dashboard export/deletion controls remain pending.
- Local PostgreSQL 17 race tests, full Go suite, vet/build, and diff checks pass, including concurrent export/update consistency, credential exclusion, cross-user isolation, deletion password failures, cascading removal, and concurrent deletion.
- The browser-fetch fix passed all GitHub CI jobs, including three production-browser tests (run 43663475819). No public deployment was performed.

## 2026-11-15 — Complete export/deletion isolation coverage

- Populated all eight exported product resources for two synthetic accounts and verified ownership in every exported row.
- Verified deletion removes every owned resource while preserving the other account's user, sessions, tracker, ratings, progress, preferences, recommendations, feedback, follows, and notifications.
- The expanded PostgreSQL race test passes locally. The account export/deletion feature commit passed all GitHub CI and CodeQL checks (CI run 43664014126, Security run 43664014129).

## 2026-11-15 — Stable session rotation and remote revocation

- Confirmed the previous main commit's CI and CodeQL are green before starting this batch.
- Added bounded active-session listing, individual remote revocation, and sign-out of all other sessions using explicit owner predicates, forced RLS, non-cacheable responses, and account-before-session locking.
- Changed refresh to rotate the credential in the same logical session record. Its ID and initial creation time remain stable, so concurrent refresh cannot escape remote revocation of a listed session ID.
- Added PostgreSQL/HTTP tests covering cross-user listing/revocation boundaries, hidden credential fields, malformed/unknown/foreign IDs, inactive sessions, listing overflow, idempotent bulk revocation, self revocation, refresh/revocation races, and mutual revocation.
- Local PostgreSQL 17 full Go race suite passes. No public deployment was performed. Dashboard controls are the next step.

## 2026-11-15 — Session-management dashboard

- Added an on-demand active-session panel with creation/last-used/expiry timestamps, a current-session indicator, individual remote revocation, and sign-out of all other sessions.
- Preserved the in-memory credential boundary and account-switch cancellation; the panel unmounts on sign-out/unauthorized responses. No device/IP metadata or credential digests are exposed.
- Added browser coverage for individual and bulk revocation, retained current-session access, disabled repeat actions, and panel cleanup on sign-out.
- Local frontend transport tests, typecheck, and production build pass. The backend session-management commit passed CI and CodeQL (runs 43685858708 and 43685858600). Browser verification for the new controls runs in GitHub CI.

## 2026-11-15 — Full-stack browser/database verification

- Added a dedicated Playwright suite against the actual Go API, production dashboard build, and PostgreSQL, with no response mocks.
- Added an isolated CI job that applies the embedded migrations and real role-provisioning script, generates a temporary masked runtime password, and runs the API as the restricted non-owner role on loopback.
- Covered persisted tracking/progress/ratings, UI-driven individual/bulk remote revocation, browser data cleanup after remote revocation, two-account isolation, foreign entry update rejection, export, and password-confirmed deletion while another account remains usable.
- Added explicit response synchronization for rating persistence and separated real/mocked browser suites. Local typecheck, test discovery, workflow YAML validation, and production build are verified; actual Chromium execution is checked in GitHub CI because the workspace disallows browser Unix sockets.
- The session-management dashboard commit passed all CI and CodeQL checks (runs 43686169885 and 43686169895). No public deployment was performed.

## 2026-11-15 — Password changes and global session invalidation

- Added authenticated password changes with current-password verification, bounded replacement hashing, strict JSON, shared authentication abuse controls, and exclusive account locking. The hash change atomically revokes every existing credential, including the caller's.
- Added dashboard controls that clear password inputs after every attempt, preserve a valid session on incorrect reauthentication, and return to sign-in after success. Only allowlisted error codes/messages reach the UI; stale error decoding cannot sign out a different account.
- Added database/HTTP tests for password validation, failed-change rollback, all-session revocation, foreign-account preservation, concurrent changes, owner-field rejection, and hashed storage. Added focused browser coverage and extended the real PostgreSQL browser flow with password change and sign-in using the replacement.
- Local transport tests, frontend typecheck/build, and the Go race suite pass. The previous full-stack browser job passed against the production-role provisioning script and PostgreSQL (CI run 43686660913). No public deployment was performed.

## 2026-11-15 — Account-panel reconciliation regression

- Browser CI detected duplicate React keys on the sibling session/password panels, which could duplicate controls across rerenders and leave stale panels after sign-out.
- Namespaced keys by panel type and account ID, preserving account-specific resets while keeping sibling identities unique.
- Added explicit one-panel-per-type assertions through authenticated rerenders and complete removal checks after remote sign-out. Both mocked and real PostgreSQL browser suites are rerun; backend/database/password race tests already passed in the preceding CI run.

## 2026-11-15 — Purpose-bound identity challenge foundation

- Added migration 000006 with forced RLS for hashed, expiring, single-use password-reset and email-verification challenges, narrowly scoped digest bootstrap, and a single pending challenge per account/purpose.
- Added challenge issuance/redemption under account-before-token locks, purpose-separated digests, email-address binding, a one-minute resend cooldown and five-issuance hourly cap, supersession, and invalidation during password changes/reset/deletion.
- Password reset revokes existing sessions and clears temporary login locks without issuing a new session. Verification records mailbox proof; it does not reset passwords or automatically authenticate. Existing session/account responses expose the current `emailVerified` flag.
- Added PostgreSQL tests for RLS, digest-only persistence, purpose confusion, expiry, replay, supersession, cooldown, address changes, password-change invalidation, deletion cascades, and concurrent single-use redemption. Raw delivery envelopes cannot be serialized to API JSON and redact structured log output.
- Runtime role/startup checks and migration/rollback CI now cover eleven private tables. The full local Go race suite passes; HTTP delivery wiring is the next step. No mail was sent to real recipients and no public deployment was performed.

## 2026-11-15 — Asynchronous recovery API and authenticated TLS mail

- Added a bounded, cancelable identity delivery worker. Recovery acceptance occurs before account lookup and SMTP, keeping public responses independent of account state/provider latency. Delivery is disabled without complete explicit configuration.
- Added strict reset/request/confirm and verification/request/confirm routes; verification recipients come only from a current verified session's stored email. Redemption remains usable during delivery outages and never authenticates automatically.
- Added an authenticated, certificate/hostname-verified implicit-TLS SMTP adapter, trusted-origin fragment links, CRLF/configuration validation, fixed public errors, and structured-log redaction. No real recipients were contacted.
- Added a separately compiled loopback-only synthetic SMTP/mailbox fixture, TLS/auth tests, queue backpressure/shutdown tests, HTTP non-enumeration/strict-body/redemption tests, recipient quotas, disabled-account cases, and reset/password-change race tests.
- The local full Go/PostgreSQL race suite, SMTP tests, vet/build, and diff checks pass. Challenge foundation CI/CodeQL passed (runs 43690416569 and 43690416496). Durable delivery retries and production abuse controls remain pending; no public deployment was performed.

## 2026-11-15 — Browser recovery and real TLS-mail CI

- Added on-demand recovery requests, current-account verification requests/status, fragment proof parsing/scrubbing, and explicit reset/verification confirmation forms. Proof never enters application request URLs or browser persistent storage; reload discards it.
- Added purpose/duplicate-field/canonical-token parsing tests and a browser assertion that loading a link does not redeem it. Account-scoped keys and API revisions keep stale proof responses from crossing account changes.
- Added a full-stack CI matrix with separate PostgreSQL baseline and identity variants, avoiding shared rate-limit state. The identity variant provisions generated/masked mail credentials, starts a loopback-only TLS mail fixture before the API, and exercises real delivery, verification, recovery, replay rejection, session revocation, and preserved private tracking data.
- Local transport/parser tests, frontend typecheck/build, suite discovery, SMTP race tests, and workflow YAML checks pass. The preceding delivery/API commit passed CI and CodeQL (runs 43691791914 and 43691791969). New browser execution is verified in GitHub CI; no public deployment or real-recipient delivery was performed.

## 2026-11-15 — Recovery startup and browser navigation corrections

- The new identity CI variant successfully exercised actual TLS mail and email verification. Its later recovery step incorrectly assumed a same-document fragment navigation would clear the signed-in browser session; the test now explicitly signs out before testing recovery while retaining an independent API session for revocation checks.
- Mail settings now fail startup if PostgreSQL is absent instead of being silently ignored by JSON development mode. Added configuration tests covering missing database, partial/valid configuration, disabled delivery, and redacted errors without sending mail.
- Existing frontend, database, PostgreSQL-browser, backend, and CodeQL jobs passed in the preceding run; the corrected mail-enabled suite is rerun.


### Account data controls

- Added authenticated dashboard export downloads through the session-bound client. Exports use a temporary browser object URL and a fixed filename; private data is not placed in browser storage or URLs sent to the API.
- Added explicit password-confirmed permanent deletion with an acknowledgement checkbox. Successful deletion clears the browser session and library; a wrong password clears the form while retaining the valid session.
- Distinguished deletion reauthentication failures from invalid sessions in the API, matching password changes. Extended real TLS-mail/PostgreSQL browser coverage to download and inspect the export, reject incorrect deletion passwords, delete the account and reject its old bearer.


### Mandatory verified-email product access

- Made email ownership mandatory for PostgreSQL tracker reads/writes and ratings. Both HTTP routing and each product repository transaction enforce the policy from live locked account state, with 403 `email_verification_required` distinct from authentication expiry.
- Kept account management, recovery, export/deletion and shared catalog access available to unverified users. Missing SMTP leaves library access locked; reset/refresh do not silently verify an address, and no existing-account bypass was introduced.
- Updated the dashboard to explain verification, disable editing, skip unverified library fetches, preserve account controls, and refresh the current profile after proof confirmation.
- Extended restricted-role PostgreSQL tests for denial, unchanged rows, reset/refresh bypass attempts, existing-session unlocking, fresh state checks and cross-account isolation. Both real browser CI suites now prove fixture address ownership through loopback TLS mail; the focused UI suite also covers unavailable delivery.


### Shared persisted login-failure budget

- Fixed unused failed-login state: failed credentialed-account logins now commit a bounded observation window/count under an exclusive account lock before returning a generic denial. Ten failures in fifteen minutes trigger a fifteen-minute temporary lock shared across instances and restarts; active locks are not extended by repeated requests.
- Added migration 000007 and fail-closed startup detection. Success/password change/recovery clears the budget; reset issuance/redemption remains possible while locked. Session issuance rechecks the credential and current profile under the same lock.
- Added separate-pool concurrent failure, threshold, expiry, successful-login reset, recovery, cross-account and generic-error tests, plus missing-migration rejection. CI applies and rolls back the new migration in order. No IP/client identifiers are collected; distributed ingress controls and lockout denial-of-access mitigation remain launch work.


### Sensitive-action reauthentication budget

- Shared the persisted password-failure budget with password changes and account deletion, preventing these confirmation endpoints from bypassing the cross-instance login limit.
- Committed failed reauthentication attempts before returning the allowlisted wrong-password error. Below threshold the existing session remains valid; active locks block sensitive actions and are never extended. Successful password changes clear the budget and still revoke all old sessions.
- Added separate-pool coverage for a window consumed across both actions, lock enforcement by login/deletion/password change, expiry, successful reset and old-session rejection.

## 2026-11-15 — Bounded identity delivery retries and session continuity

- Added three-attempt SMTP delivery with one-/two-second backoffs, per-attempt timeouts capped by proof expiry, and cancellation-aware shutdown. Retry the same issued proof rather than invalidating it or consuming resend quotas; successful delivery stops immediately. Provider failures remain redacted.
- Added tests for eventual success, retry exhaustion, unchanged proof, expiry, cancellation, deadlines, and the service issuing only one challenge across multiple deliveries.
- Added `docs/PROJECT_CONTEXT.md` as the GitHub continuation entry point, linked from README. It records architecture, completed versus schema-only features, security constraints, known issues, next priorities and exact verification/resume steps. Corrected stale architecture/security wording alongside it. No ChatGPT Project file/instruction editing capability was available.
- Local full race suite passed against a fresh disposable PostgreSQL 17 instance (restricted-role integration suite included); vet/build and whitespace checks passed. The initial reused database instance had stale process state; isolated reprovisioning resolved the environment failure. This commit's GitHub CI and CodeQL are checked after publication.
- Durable restart-safe delivery remains pending: queue/retry state is volatile, network ambiguity can duplicate mail, and a single slow worker can fill the bounded queue. Nothing deployed; no real mail or personal data used.

### Verified continuation checkpoint

- Commit `243b759` passed all five CI jobs (run 43697767885), including both real PostgreSQL/browser variants and database migration/isolation checks. Both CodeQL analyses passed (run 43697767176).
- Added root `AGENTS.md` to direct future sessions to the GitHub context and require updates alongside substantive work, while preserving implementation/test priority and the existing security boundary. Updated the context with exact verified implementation links.

## 2026-11-15 — Encrypted durable identity outbox

- Added migration 000008 with a forced-RLS encrypted outbox, ownership-bound challenge foreign key, persisted retry scheduling, bounded attempts and hashed expiring leases. Runtime now requires twelve forced-RLS tables; role provisioning and CI migration/rollback assertions include the outbox.
- Proof issuance and AES-256-GCM encrypted envelope insertion commit atomically. A separate explicit `MAIL_OUTBOX_KEY` lives outside PostgreSQL; envelope authentication binds challenge ID/version, and claims recheck live account/purpose/digest/address/expiry before sending. Mail configuration fails closed without a valid key.
- Configured API instances poll persisted deliveries, release all database locks before SMTP, and acknowledge only the matching lease. Restart recovery does not issue another proof or consume resend quota. Five claims include crashed attempts, with persisted 30/60/120/240-second backoffs; completed/exhausted/invalid/expired ciphertext is erased, and completed metadata is purged after one day in bounded batches.
- Added tests for encryption randomness, tampering, row substitution, wrong keys, atomic rollback, fresh-pool restart, lease exclusion/recovery, stale acknowledgements, quotas, RLS visibility, invalid proof/account states, deletion, attempt exhaustion, service restart and metadata retention. Full local PostgreSQL race suite, vet/build and diff checks passed. GitHub CI/CodeQL are verified after publication.
- HTTP 202 still acknowledges a bounded volatile pre-issuance queue. Key-ring rotation, durable request acceptance and operational monitoring remain open; SMTP acceptance/acknowledgement ambiguity can duplicate mail. Nothing deployed; only synthetic accounts and local mail fixtures were used.

### Verified encrypted-outbox checkpoint

Commit `cb428e0` passed all five CI jobs (run 43699049145), including real PostgreSQL browser, TLS-mail recovery, forced-RLS isolation, migration apply/rollback, and frontend checks. Both CodeQL analyses passed (run 43699049052). Updated the continuation context to this verified implementation. Remaining priorities are durable pre-issuance acceptance, key-ring rotation/operations, monitoring and distributed ingress abuse controls; no deployment was performed.

## 2026-11-15 — Durable identity request acceptance

- Added migration 000009 with a forced-RLS encrypted request inbox; runtime requires thirteen secured tables, and role grants plus CI apply/rollback assertions include it. Configured mail now commits an intent before HTTP 202, with no volatile fallback and no account lookup in admission.
- Added fixed-size 540-byte authenticated request envelopes under a distinct row-bound domain. Reset intents carry submitted email; verification intents carry only an encrypted session digest, never raw bearer tokens. Shared advisory-lock admission caps active intents at 256 across pools/instances; requests expire after fifteen minutes.
- Added leased issuance workers, five persisted claims with 5/10/20/40-second backoff, expiry/exhaustion cleanup and one-day completed-metadata retention. Inbox completion, challenge issuance and encrypted mail insertion commit atomically; failures roll them all back. Existing account/session checks revalidate queued verification, and lock ordering keeps account/session before request/proof/outbox rows.
- Added tests for account-independent acceptance with user-table reads revoked, fresh-pool recovery, atomic rollback/retry, digest-only verification intents, logout/refresh/password-change/deletion/expiry rechecks, shared concurrent capacity, active/stale lease fencing, generic HTTP database failures, wrong-key preservation, attempt exhaustion, retention, and intake cancellation/shutdown.
- Full local Go race suite passed with PostgreSQL integration tests enabled (85 seconds for database package); vet/build and whitespace checks passed. An expiry fixture initially violated the existing session timestamp constraint; corrected its creation timestamp and reran the complete suite successfully. GitHub CI/CodeQL are checked after publication.
- Updated OpenAPI, runtime/security guidance, roadmap and `PROJECT_CONTEXT.md`. Key-ring rotation, queue monitoring, ingress abuse controls and independent security review remain open. Anonymous pending reset intents can remain encrypted until processing/expiry after deletion because admission has no owner lookup. Nothing deployed; only synthetic data/local mail fixtures were used.

## 2026-11-15 — Bounded identity keyring and planned rolling rotation

- Verified durable-admission commit `e627997`: all five CI jobs passed (run 43700420364), including the real PostgreSQL/TLS-mail browser suites and migration/isolation checks; both CodeQL analyses passed (run 43700420343).
- Added `MAIL_OUTBOX_PREVIOUS_KEYS` with at most three distinct canonical base64 32-byte decryption keys. Encryption always uses the primary; inbox/outbox decryption tries the bounded ring while preserving domain/row authentication. Existing ciphertext/schema remains compatible. Invalid, duplicate, noncanonical or excessive keys fail startup with fixed errors; key formatting is redacted.
- Added configuration/cipher tests and a real PostgreSQL mixed-key rollout test covering old inbox/outbox work, missing-key preservation, new-primary writes, proof redemption and prepared old-primary readers. Browser CI now provisions an additional generated/masked decryption key to exercise runtime configuration.
- Added `IDENTITY_KEY_ROTATION.md` with prepare-readers/switch-writers/retire stages, conservative timestamp-based retirement, rollback and key-compromise limitations. Updated runtime/security/roadmap/project context alongside it; automated secret rollout, monitoring, backup/restore exercises and independent review remain open.
- Local full Go race suite passed with PostgreSQL integration enabled (83.5 seconds for database package); vet/build and whitespace checks passed. Restored temporary Go/PostgreSQL tools after the session environment reset. GitHub CI/CodeQL are checked after publication. Nothing deployed; only synthetic data and local mail fixtures were used.

## 2026-11-15 — Read-only identity queue diagnostics

- Verified keyring commit `65ed88e`: all five CI jobs passed (run 43710679593), and both CodeQL analyses passed (run 43710679512).
- Added `cmd/identity-status` and a restricted-role aggregate repository method. A single read-only repeatable-read transaction reports versioned queue counts, leases, delays, attempted-active work, cleanup needs, retained payload counts and age/expiry timestamps. No recipients, account/row IDs, credentials/digests/leases, ciphertext or connection URLs are returned. No keys, SMTP connection or HTTP route are required.
- Shared the admission-capacity constant with diagnostics. CLI initialization/query/output errors are fixed messages, and a successful exit means a snapshot read, not provider health. The command does not start workers or mutate/clean records.
- Added synthetic PostgreSQL fixtures for ready/leased/delayed/expired/attempt-limited/completed states, empty snapshots, JSON privacy checks and unchanged request records; added CLI configuration redaction tests. Full local Go/PostgreSQL race suite passed (86.4 seconds for database package), and the final diagnostic field naming passed a focused race rerun. Vet/build and whitespace checks passed.
- Added `IDENTITY_QUEUE_OPERATIONS.md` and updated README, runtime guide, roadmap and project context. Counts intentionally overlap; completed records include discarded/expired work and are not successful-delivery counts. Monitoring export, alerts, outcome/provider telemetry, recovery exercises and independent review remain open. GitHub CI/CodeQL are verified after publication; nothing deployed.

## 2026-11-15 — Bounded identity queue terminal outcomes

- Verified diagnostic commit `ceedc45`: all five CI jobs passed (run 43711276451), with both CodeQL analyses passing (run 43711276432).
- Added migration 000010 with separate bounded outcome allowlists for request/mail queues. Historical completed records backfill as `LEGACY_UNKNOWN`; compatible old writers can leave NULL completion outcomes, also classified unknown. Backfill uses trusted queue context under forced RLS and is tested with a non-superuser table owner. Startup fails closed when either outcome column is missing.
- Workers now atomically record actual terminal results alongside completion/payload erasure: request issuance/no-action, sender acceptance, invalidation, expiry and exhaustion. Generic HTTP acceptance and merged no-action reasons preserve the existing privacy boundary. Lease fencing prevents stale acknowledgements changing newer work. Wrong-key failures and ordinary retries remain nonterminal.
- Read-only diagnostics adds retained outcome counts. Successful fifth claims remain accepted/issued, never inferred exhaustion. Counts decrease with retention/cascades and must be treated as gauges, not cumulative success counters; SMTP acceptance is not confirmed mailbox delivery.
- Added synthetic result/diagnostic assertions, constraints, missing-schema rejection and legacy migration tests. Full local PostgreSQL race suite passed (103.3 seconds for database package), along with vet/build and diff checks. CI migration/rollback order and runtime/security/project context were updated; GitHub CI/CodeQL are checked after publication. Nothing deployed.

## 2026-11-15 — Prometheus diagnostic text export

- Verified outcome commit `1df9e17`: all five CI jobs passed (run 43712223213), including migration/backfill/rollback, PostgreSQL isolation and real browser/mail flows; both CodeQL analyses passed (run 43712223198).
- Added `identity-status -format=prometheus`, preserving JSON as default. Fourteen gauge families/38 fixed series report the same aggregate snapshot using only fixed queue/outcome labels. Snapshot time, capacity, age and retained outcomes are exported without private data, keys, provider text, an HTTP listener or automatic scheduling.
- Added tests for metric metadata/cardinality/numeric syntax, sample values, future/empty ages, writer errors/short writes, JSON compatibility and argument/help handling. Invalid arguments are never echoed, and database errors retain fixed messages. Local race tests, vet/build and diff checks pass; database tests in the exporter-only local suite skipped without TEST_DATABASE_URL, while the preceding full PostgreSQL outcome suite and its CI passed. Final GitHub CI reruns all integration/browser checks.
- Updated operations/runtime/project context with exact metric names, atomic textfile replacement, freshness checks, shared-database deduplication and gauge semantics. Collector scheduling, missing-scrape alerts, provider instrumentation, recovery exercises and independent review remain open; nothing deployed.

## 2026-11-15 — Atomic monitoring snapshots and tested alert templates

- Verified metric-export commit `2318f8d`: all five CI jobs passed (run 43712857218), with both CodeQL analyses passing (run 43712857181).
- Added `identity-status -format=prometheus -output-file=/absolute/path.prom`. Snapshot output is staged in the same directory, written/synced/closed with 0640 permissions, then atomically renamed. Pre-rename failures preserve the prior export; staging files are removed and existing destination symlinks are replaced without following their targets. File/argument errors remain fixed messages.
- Added disabled hardened systemd oneshot/timer and loopback Prometheus examples. Opt-in alert rules cover target availability, per-target missing/stale/future snapshots, capacity/age, cleanup and retained exhaustion. State alerts require a fresh reachable snapshot; outcome rules use gauges, never counter rates. No receiver/routing or live configuration is installed.
- Added checksum-pinned Prometheus 3.13.5 LTS tooling and a monitoring CI job checking actual exporter output, configuration, alert behavior and inactive unit syntax. Rule tests cover healthy/empty queues, per-target absence/disappearance, stale/future suppression, multi-target label matching, hold times and recovery. Testing caught/fixed a dropped queue label in the capacity ratio using explicit group-left matching.
- Added atomic-file permission/content/symlink/failure tests. Both real-PostgreSQL browser CI variants now run the actual CLI export with the restricted role and check output existence/permissions. Local Go race tests, vet/build, promtool parser/config/rule tests and systemd syntax checks pass; local database tests skipped without TEST_DATABASE_URL in this session and are rerun in CI.
- Updated monitoring/operations/runtime/roadmap/project context with activation prerequisites, clock/freshness limits, target-inventory gaps and retained-outcome semantics. Nothing deployed or activated. Host-specific review, credentials/users, target inventory, receiver routing, recovery exercises and security review remain open.

## 2026-11-15 — Request-log privacy and live readiness

- Verified monitoring commit `b8a02c0`: all six CI jobs passed (run 43727756412), with both CodeQL analyses passing (run 43727756357).
- Replaced raw URL path logging with matched ServeMux route templates or `unmatched`; nonstandard HTTP methods become `OTHER`. Header/body/query/path-parameter values never enter middleware log fields. Tests cover matched IDs, unknown routes, redirects and custom methods containing synthetic secrets.
- Added separate `/readyz` endpoints with generic non-cacheable 200/503 responses, a two-second/caller deadline, and one in-flight database probe per handler. `/healthz` remains process liveness. PostgreSQL readiness rechecks the live restricted role, thirteen forced-RLS tables and required security columns; startup reuses the same validation path. Local JSON readiness has no external dependency.
- Added tests for readiness redaction, cancellation, overlapping probes, live RLS/role/schema drift, closed pools, schema-lock deadlines and recovery while liveness remains independent. Full local Go race suite passed with PostgreSQL integration enabled (80.8 seconds for database package), plus vet/build and whitespace checks.
- Updated OpenAPI, runtime/security guidance, roadmap and project context. Readiness is a momentary minimum-safeguard/dependency probe, not a full policy audit, mail check or public-launch approval. Trusted ingress, distributed abuse controls, recovery exercises and independent review remain open. GitHub CI/CodeQL are checked after publication; nothing deployed.

## 2026-11-15 — Isolated logical recovery and offline credential fencing

- Verified readiness commit `2274261`: all six CI jobs passed (run 43729286206), with both CodeQL analyses passing (run 43729286415).
- Added an opt-in real PostgreSQL 17 `pg_dump`/`pg_restore` drill. A randomly provisioned synthetic source is dumped into a private temporary archive and restored into two fresh databases, never the supplied provisioning database. Archives exclude ownership/ACLs; the drill restores the migration ledger, checks checksums, reapplies the production grant script to a restricted synthetic role, verifies readiness, all eight populated export resources, owner isolation and direct forced RLS on all thirteen private tables.
- Tested historical snapshot hazards with a post-backup password change/account deletion. Missing old queue keys fail closed without losing work; a compatible rotated keyring resumes old inbox/mail, preserves issued tokens and enforces single-use redemption. Only in-memory synthetic senders are used.
- Added manual `deployments/recovery-fence.sql`, deliberately outside migrations: offline administrator/RLS-bypass guard, exclusive locks, bounded lock/statement waits, atomic reset-only login locks, session/proof revocation and complete restored queue deletion. Tests cover restricted-role rejection, injected mid-script rollback, repeated fencing, disabled accounts, rejected historical credentials, deletion reconciliation and fresh individual recovery under a new key with preserved product data.
- Added a dedicated PostgreSQL 17 container CI recovery job with matching clients/server and race checks; no archive/keys are uploaded. Updated backup/key/runtime/security guidance, roadmap and continuation context. The runbook explicitly requires independent post-snapshot deletion/address/security reconciliation before reopening even recovery intake; no such inventory, real backup service, PITR or measured recovery objective is implemented.
- Local focused recovery race test passed, then the complete Go race suite passed with PostgreSQL integration and recovery enabled (86.7 seconds for database package); vet/build and whitespace checks passed. The workspace isolates networking between commands, so PostgreSQL and its integration suite run in the same command lifecycle. GitHub CI/CodeQL are checked after publication. Nothing deployed or activated; all accounts/data/keys were synthetic.

## 2026-11-15 — Consistent API response privacy headers

- Verified recovery commit `fa8e014`: all seven CI jobs passed (43730865833) and both CodeQL analyses passed (43730865745).
- Authentication, protected account/product and identity handlers already set no-store. Extended that policy to the shared outer middleware for both routers, including local mode, unmatched/method errors, redirects, HEAD and CORS preflights. Added nosniff and no-referrer response policies. Public catalog caching is intentionally disabled; dashboard document policies and clearing historical caches remain separate concerns.
- Added regression coverage for both routers and malformed login/registration requests, without touching database state. The redirect fixture was corrected to Go 1.26's actual 307 response. Updated runtime and continuation guidance.
- Local Go tests/race tests, vet and build passed; PostgreSQL/recovery tests skip locally without the disposable database and run in CI. Final exact-commit CI/CodeQL are verified after publication. No deployment or configuration activation.

## 2026-11-15 — Transactional library removal API

- Verified `0393fbc`: all seven CI jobs (43742215931) and both CodeQL analyses (43742215978) passed.
- Added DELETE tracker endpoint and request-aware repository removal for both runtimes. PostgreSQL authenticates/verifies ownership, locks/removes the owned tracker row and deletes its title ratings/progress in one transaction; foreign/missing/repeated IDs share 404. JSON restores its in-memory entry on persistence failure. Re-adding starts fresh; catalog, follows and recommendation history are preserved.
- Updated reviewed runtime DELETE grants (reapply for existing installations), OpenAPI, roadmap and continuation context. Dashboard controls remain pending. No schema migration or deployment.
- Added PostgreSQL ownership/auth/verification, injected privilege-failure rollback, HTTP status/empty-body, cleanup and re-add tests. Added JSON persistence-failure/restart tests. Local unit/race tests, vet/build passed; PostgreSQL tests skip locally without TEST_DATABASE_URL and run in CI. Exact-commit CI/CodeQL are verified after publication. Synthetic fixtures only.

## 2026-11-15 — Dashboard library removal

- Verified API commit `b38a78c`: all seven CI jobs passed (43743233860), with both CodeQL analyses passing (43743233912).
- Added per-title confirmation explaining rating/progress deletion, cancel/retry and disabled pending controls. Removal sends DELETE without a JSON body, handles 204 and refreshes library totals/catalog availability. Failure preserves the entry and confirmation. Account changes clear confirmations; mutation completion and overlapping refresh revisions prevent stale UI writes.
- Added mocked browser cancellation/failure/retry/re-add checks and account-switch confirmation cleanup; added a delayed DELETE 204 transport isolation regression. Extended the real PostgreSQL browser workflow with removal/cancel/re-add, fresh rating/progress and another owner's unchanged rating.
- Local frontend tests, type checking and production build passed. Local browser launch was blocked by missing Chromium; attempted installation returned invalid archives. Browser workflows are verified in CI against mocked transport and real PostgreSQL. Updated roadmap, runtime and continuation docs. Existing deployments still require the previous API batch's reviewed DELETE grants. Nothing deployed or activated; synthetic fixtures only.

- First browser CI caught an ambiguous alert selector matching Next.js's route announcer as well as the application error. Scoped the regression to the main dashboard; the application showed the intended generic failure and retained its entry. The follow-up commit reruns all checks before completion.

## 2026-11-15 — Catalog feed validation and release comparison

Implemented bounded strict JSON snapshot decoding, provider-scoped identities, rich metadata, typed title/season relations, nullable totals and episode schedules by release track. Added dry-run catalog-check CLI and tests for malformed/oversized input, invalid metadata, duplicates, unknown totals, schedule-versus-release semantics, baseline suppression, timestamp corrections, stale snapshots, track isolation and stable ordering. CATALOG_FEEDS.md records remaining persistence/provider boundaries.

No network provider, database ingestion, user progress mutation or notification delivery is enabled. Durable event-key retention, canonical resolution and owner-scoped delivery remain pending.

Validation: existing main 5d5b7d2 CI 43745027034 and Security 43745027043 inspected: success. Local tests not run: no Go toolchain and shell DNS unavailable. New implementation CI pending publication; baseline CI does not validate this change.

## 2026-11-15 — Runtime catalog metadata

Public PostgreSQL catalog list/detail now return existing synopsis, airing season and cover URL columns. Added episodeCountKnown alongside the existing numeric episodeCount to distinguish NULL/unknown from known zero without changing the existing numeric field. Local starter titles explicitly mark known totals. No schema migration or new runtime grants are required.

Added restricted-role PostgreSQL and anonymous HTTP regression coverage for metadata parity, NULL fields, known-zero counts and missing titles, plus local seed count coverage. Local execution unavailable (Go absent); exact-commit Actions pending.

## 2026-11-15 — Catalog validation checkpoint and continuity constraints

Verified exact-commit Actions:
- 51da5bc: CI 43746409535, all seven jobs passed; Security 43746409597 passed.
- 2b6781e: CI 43746527946, all seven jobs passed, including restricted PostgreSQL integration/migration tests, restore drill, frontend/mocked-browser tests and both full-stack browser suites. Backend includes unit/race/vet/build/vulnerability checks.

Follow-up adds cycle detection for normalized PREQUEL/SEQUEL edges, with reciprocal-relation, two/three-node cycle, side-story and partial-graph tests. Snapshot validation only sees submitted edges; persistence must additionally check the full canonical graph. Added OpenAPI catalog list/detail schemas, including metadata and episodeCountKnown. Follow-up exact-commit CI pending; local toolchain unavailable. No deployments.

## 2026-11-15 — Durable catalog ingestion

Added migration 000011, administrator-only catalog-import command and transactional catalogsync implementation. Enabled/policy-bearing source registration is required; no external source is activated. Exact provider IDs resolve canonically without name matching. A single authoritative source owns each imported title; conflicts fail closed. Structured latest provenance, per-source high-water marks, schedule revisions, retained relation resolution and canonical unique release events commit together. Full persisted continuity cycles roll back the batch. Partial feeds preserve confirmed releases; baseline and historical backfills do not flood alerts.

Added PostgreSQL coverage for source disablement, stale/repeated feeds, runtime-write denial, canonical visibility, schedule/correction history, event deduplication across partial feeds, conflict rollback and delayed relation resolution. Updated migration application/rollback CI and restore-ledger count.

Validation: baseline 42111df CI 43746726043 and Security 43746726090 inspected: passed. New ingestion code pending exact-commit Actions. Local Go unavailable in this session; no local test pass claimed. No deployment or provider activation.

## 2026-11-15 — Public catalog details and episode schedules

Added a consistent repeatable-read details endpoint with latest structured metadata/source timestamp, genres/tags/studios/synonyms, resolved canonical season/title relations and latest episode schedule revisions by track. Responses contain only public catalog data; tags/synopses may contain spoilers. Added local-mode empty enrichment and explicit arrays. API role gains read-only catalog tables and only source id/provider-key columns. Readiness now checks migration 000011 and required read grants. Reapply runtime-role.sql before starting an updated runtime.

Tests cover anonymous HTTP metadata, source timestamps, schedule-versus-release semantics, canonical relations, seeded empty enrichment and invalid/missing identifiers. OpenAPI updated. Exact-commit CI pending; local Go unavailable. No deployment.

## 2026-11-15 — Owner-scoped confirmed-release subscriptions and notices

Verified: ingestion 8ee67a3 passed all seven CI jobs (43751739458) and both CodeQL analyses (43751739539); catalog details ca4e3a8 passed all seven CI jobs (43751994428) and both CodeQL analyses (43751994391).

Added migration 000012 with release-track preferences and unique owned event/channel notifications. Verified-account follow/list/unfollow, bounded polling materialization and idempotent read endpoints use account/session locks, explicit ownership and existing forced RLS. Notifications materialize only eligible confirmed episode events after subscription creation. No background mail/push, franchise follow or public deployment is enabled.

Added importer-to-notification integration coverage, SUB/DUB/ALL filtering, replay suppression, read ownership, concurrent-session deduplication, unsubscribe isolation, export isolation and HTTP authentication/verified-email/owner-override checks. Expanded runtime grants, readiness, migration/rollback CI and recovery ledger count. Exact-commit validation pending; local Go unavailable.

## 2026-11-15 — Dashboard release-alert controls

Added a verified-account release panel with follow/unfollow, per-title track preferences, read state and bounded polling only while open. Account-keyed remounts, request revisions and unmount guards fence late responses. Credentials/notices stay in memory. The panel states polling/response limits and has manual refresh/retry.

Added mocked browser coverage for preference requests, follow/removal, read state, account reset, storage exclusion and delayed prior-account notification results. Existing real PostgreSQL/browser suites remain required. Read web/AGENTS.md; packaged Next guides were unavailable in this empty workspace, so consulted the current official use-client guide before using the existing client-component pattern. No framework conventions or dependencies changed. Exact-commit CI pending; no local Node/Go toolchains available.

## 2026-11-15 — Exact episode selections

Added migration 000013 with nullable exact episode-number sets in the existing forced-RLS progress table and a database consistency constraint. Verified owner-scoped GET/PUT routes explicitly distinguish aggregate counts from ordered unique exact selections. Status-only changes preserve exact data; explicit count mutations switch modes. Library exposes exactProgress, export includes numbers and existing deletion cascades apply.

Added tests for gaps/order, count-mode boundaries, known-total validation, duplicates/invalid sets, empty selections, foreign IDs, status preservation, export, deletion, database constraint enforcement and HTTP authentication/verification/strict JSON. Updated migration rollback/restore count and readiness. Exact-commit CI pending; no local toolchains or deployment.

## 2026-11-15 — Exact episode dashboard editing

Added explicit count-to-exact replacement UI, comma-separated gap-aware selections, known-total validation, clear/cancel/retry and unmount/revision fencing. Exact-mode library cards disable the aggregate +1 shortcut. Added parser unit tests and browser coverage for replacement semantics, sorted gaps, duplicate rejection and clearing. No dependencies/framework APIs changed. Exact-commit CI pending; local toolchains unavailable.

## 2026-11-15 — Provider format integration and approval boundaries

Added bounded offline Jikan v4 metadata/episode-page conversion into the existing neutral feed, a catalog-convert CLI requiring actual observation time, and synthetic mapping/error tests. Normalizes names, aliases, synopsis, type/status/year/quarter, unknown totals, genres/themes/demographics/studios and supported anime relations. Aired dates become UNKNOWN-track schedules only; no confirmed availability is fabricated.

Recorded primary-source capability/approval review in PROVIDERS.md. AniList's current terms restrict competing trackers/mass collection; Jikan's linked terms were inaccessible and source redistribution rights remain unverified. No network adapter/source activation or live catalog API requests were performed. Exact-commit CI pending; local toolchains unavailable.

## 2026-11-15 — Linked-season follows and announcement events

Added explicit ANIME/FRANCHISE follow scopes and transitive prequel/sequel membership. Scope changes replace the former mode for the title and preserve follow time. New post-baseline ANNOUNCED titles/UNKNOWN transitions create unique canonical announcement events; polls distinguish announcements from confirmed episode availability. SQL deduplicates multiple graph paths/follows, uses a fixed two-second statement timeout and serializes subscription changes/materialization with an account advisory lock after existing account/session locks. New subscriptions are capped at 1000 titles, with existing preference changes preserved.

Dashboard and OpenAPI expose scope selection. Added end-to-end database coverage for delayed/transitive linked seasons, announcement/episode distinctions, single-season isolation, repeat polls, scope replacement and quotas; browser scope preference coverage updated. No network source or deployment enabled. Exact-commit CI pending.

## 2026-11-15 — Cross-title freshness fix and real-browser integration

Review found that comparing a release against the provider-wide high-water mark could suppress a title's eligible release after a newer partial update for a different title. Comparison now uses the prior observation for that title; the provider high-water mark still rejects stale batches and first-source imports still baseline history. Added a specific multi-title partial-feed regression.

Added real PostgreSQL/browser coverage for exact episode replacement, foreign read/write rejection, franchise DUB preferences, stopping follows, account-switch UI reset and browser-storage exclusion. Synchronized README, architecture and runtime upgrade documentation.

Verified predecessor checkpoints: 4ee3076 CI 43752308412/Security 43752308409; 9b7ef74 CI 43752564635/Security 43752564633; 02221f7 CI 43752871302/Security 43752871261; b45ab83 CI 43753082945/Security 43753082990; 75b44ca CI 43753443802/Security 43753443843; af6dd27 CI 43753824957/Security 43753825012. All seven CI jobs and both CodeQL analyses passed for each inspected SHA. Follow-up CI pending publication. No local test pass, approved live source or deployment claimed.

## 2026-11-15 — Real-browser throttle handling and test discovery correction

Inspected 2a54c1b CI 43754113848: PostgreSQL-browser failed at the new test's sign-in; all other CI jobs and CodeQL passed. Downloaded the synthetic browser error context and confirmed the dashboard's explicit "Too many attempts" response. The real API's shared ten-token/one-per-six-second authentication budget was depleted by the preceding test and fixture setup. Added bounded login retries only for actual HTTP 429, honoring asserted Retry-After headers; all non-200/non-429 statuses still fail immediately. Production throttling is unchanged.

Audit also found the mocked-browser configuration selected only auth.spec.ts. New release/episode files were typechecked but not executed by earlier green CI. Updated testMatch to include both files explicitly. Previous broad statements about new mocked-browser coverage must be read with this correction. Exact-commit corrected discovery and real-browser run pending; no local test pass claimed.

## 2026-11-15 — Imported catalog discovery and spoiler-aware details

The dashboard no longer limits search to its initial catalog page: debounced server searches return matching imported titles and aliases/native names, while request revisions reject stale queries. PostgreSQL search includes imported synonyms under the public read boundary. Search results and library cards can open sourced metadata, genres/studios/aliases, canonical relation navigation and current schedule/confirmation revisions. Synopsis/tags require explicit spoiler reveal; schedule text does not imply availability. Unknown totals remain explicit.

Added imported-alias database coverage and a mocked browser test for a title outside the initial page, metadata/provenance, unknown totals, schedule-versus-confirmation labels and spoiler reveal/hide. Non-real browser discovery now automatically includes spec files while excluding the two real API suites. Predecessor corrected browser selection ran 11 mocked tests successfully; its real PostgreSQL/browser status must also be inspected. New changes pending exact-commit CI.

## 2026-11-15 — CC0 live factual metadata fallback

Verified f8901e6: all seven CI jobs 43756443646 passed, with 12 explicitly executed mocked browser tests and both real browser suites; both CodeQL analyses 43756443693 passed.

Primary-source review identified Wikidata structured facts as CC0. Added an explicit-item live fetch command and bounded fixed-endpoint client for direct anime-TV entities, selected labels/aliases/genres/publication year/episode totals, rank handling and ambiguous-total preservation. Added process-wide one-second pacing, maxlag, request/response bounds, redirect refusal, cooldown/advice for Retry-After and no automatic retries. It reads no user accounts, prose/media or streaming data and does not infer availability.

One manual primary-API smoke check returned HTTP 200 and the expected direct anime-TV class for Q124570859; no database import or source activation occurred. Added synthetic transport/mapping/security/error tests. Broad coverage, scheduled fetching/caching, studios/relations and confirmed availability remain pending. New code requires exact-commit CI; no local project test pass claimed.

## 2026-11-15 — Fixture setup respects the shared auth budget

29e30ac passed all Go/provider tests, database integration, frontend, monitoring, recovery, identity-browser and both CodeQL analyses (CI 43757449478 / Security 43757449486). PostgreSQL-browser failed because fixture registration received an explicit HTTP 429 after the preceding real test. The earlier bounded retry helper covered browser sign-in but not setup. Extended response-aware Retry-After handling to new fixture registration/verification calls, preserving hard failures for every non-429 status and all existing assertions. Application limits are unchanged. Exact-commit CI pending.

## 2026-11-15 — First release for a newly discovered season

Review found that treating every new title as a baseline could hide the first confirmed episode of a sequel that first appears already releasing. First provider imports still baseline all history. On later batches, new titles now compare the release time with the prior provider observation, while known titles retain per-title comparison. Historical newly imported titles still produce no release event. Added a two-title regression proving the new first release is retained and the historical backfill suppressed. Exact-commit CI pending.

Verified predecessor 0f1dc2f: all seven CI jobs 43757851776 and both CodeQL analyses 43757851574 passed, including the response-aware fixture setup and real browser feature coverage.

## 2026-11-15 — Public catalog freshness diagnostics

Added catalog-status using a read-only repeatable-read snapshot and separate explicit connection configuration. Reports provider age, enabled/policy-recorded status, never-synced/future observations, stale counts and latest public schedule/confirmation totals. It never reads account resources or fetches network data. Added an optional SELECT-only catalog-reader role and an operations guide distinguishing provider-wide partial-feed timestamps from per-title/upstream freshness.

Added PostgreSQL regression coverage for stale/never/disabled/future sources, threshold changes, latest schedule counts and restricted-role reads. No service/receiver/provider activated. Exact-commit CI pending; shell toolchains unavailable.

Verified predecessor b5b07f9: all seven CI jobs 43758257305 and both CodeQL analyses 43758257358 passed, including the new-season first-release regression. Freshness diagnostics remain pending exact-commit publication/checks.

## 2026-11-15 — Verified catalog/tracking/release checkpoint

ac7483b passed all seven CI jobs in 43758565298 and both CodeQL analyses in 43758565307. Inspected job results and browser summaries: 12 mocked tests and two real PostgreSQL/browser tests passed, alongside the dedicated identity-browser suite, Go unit/race/vet/build/vulnerability checks, PostgreSQL 17 integration/migration/rollback, recovery and monitoring checks.

Fifteen implementation/test commits from 42111df through ac7483b cover this continuation. The repository remains public and no deployment, host service, scheduled provider or real-user data import was activated. One bounded Wikidata primary-API smoke request succeeded; automated provider tests use synthetic transports. Complete source/availability coverage, scheduled freshness, cross-provider arbitration, recommendation/AI and production operations remain open. This final documentation update records the verified implementation SHA; its own automatic checks may run separately.

## 2026-11-16 — Owner-scoped evidence-backed recommendation API

Added verified-account declared feature preference list/update/reset and PostgreSQL Personal Match serving. Reads use consistent snapshots, authenticated owner predicates and existing forced RLS. Explicit preferences and up to 1000 own ratings feed the ranking core; up to 500 title-ordered candidates exclude every owned tracker entry. Only referenced current CURATED/SERIES/NONE observations on active non-spoiler-sensitive features participate. Missing evidence yields an empty list, not invented matches. Responses expose bounded feature-key evidence, not raw references/free text or user IDs. Deterministic ties now use feature keys/canonical IDs.

Added synthetic database/HTTP tests for cold start, owner separation, rating inference, tracker exclusion, preference reset, spoiler/provider/unreferenced filtering and strict request bodies; ranking tie regression added. Runtime-role grants now include read-only feature evidence and owner-scoped preference writes. Reapply the role script on existing installations. No migrations, personal history, external AI calls or deployment added.

Baseline f90800a CI 43758880391 and Security 43758880408 inspected: success. New code pending exact-commit CI; shell toolchains unavailable, so no local test pass claimed.

## 2026-11-16 — Opt-in factual catalog worker with separate write role

Recommendation/preferences 08c6ccc passed CI 43759574712 and Security 43759574718. Added explicit Wikidata worker configuration, bounded non-overlapping cycles, cancellation and full-interval retry behavior. Approval and persisted source observation time are checked before HTTP; a session advisory lock suppresses simultaneous collectors. Import rechecks approval and commits atomically. Default worker remains idle without configuration.

Added a dedicated catalog writer role that cannot enable sources, remap external identities or read/write private tables. CheckWriter rejects elevated/owner/member roles and even private column grants. Narrow label/URL update grants support PostgreSQL row-lock requirements. Added synthetic configuration/scheduler tests and disposable PostgreSQL role, approval, lock, import and persisted-due tests. No real source, scheduler host or deployment activated. Exact-commit CI pending; no local test pass claimed.

## 2026-11-16 — Personal Match dashboard

Worker 0a1c0c5 passed all seven CI jobs 43760194080 and both CodeQL analyses 43760194072. Added verified-account preference editing/reset, compatibility evidence and honest no-evidence results in an account-keyed client component. Unmount/request revisions and the existing in-memory session client fence stale responses.

Added two automatically discovered mocked browser tests for save/reset, cold start, storage exclusion and delayed prior-account results. Extended the real PostgreSQL/browser feature test to verify preference persistence/foreign-account isolation and unsupported-match handling. No personal history or fabricated production observations seeded; no external AI calls/deployment. Exact-commit CI pending. Existing client-component guide applies; packaged framework guides/toolchains remain unavailable in this workspace.

## 2026-11-16 — Reviewed public feature ingestion

Added strict offline feature-import with a separate explicit curation connection, no HTTP/LLM calls and no personal/profile fields. Validates bounded snapshots, existing catalog IDs, active non-spoiler taxonomy, finite four-decimal values/confidence and credential/query-free HTTPS references. Reference syntax is not a rights/factuality check; editor review remains mandatory.

Imports share catalog locking, append observation history, replay identical snapshots without duplicates and roll back same-time conflicts or any invalid record. Added decoder boundary and disposable PostgreSQL idempotency/conflict/rollback/API-write-denial tests, plus curation-to-recommendation integration. No curated production data or personal history imported. Exact-commit CI pending; no local toolchain pass claimed.

## 2026-11-16 — Persisted worker admission and provider cooldowns

Verified curation 9057779: CI 43773086129 and Security 43773085982 passed all seven CI jobs and both CodeQL analyses. Added migration 000014 with public source-attempt metadata only. Admission persists STARTED/minimum next-attempt time before HTTP, bounding restart/crash retries. Fixed outcomes and next-allowed times persist success, fetch/import failure and provider rate limits. Longer Retry-After values are no longer shortened to an hour; numeric overflow saturates instead of causing an early retry.

Added worker restart/crash/provider-delay tests, long/HTTP-date/overflow header tests and attempt fields to read-only diagnostics. Updated catalog role grants, migration apply/rollback and restore ledger count. No private table/RLS changes, raw response/error storage, item-selection persistence, live activation or deployment. DB loss after HTTP may still prevent recording a longer cooldown; admission's minimum interval remains durable. Exact-commit validation pending.

## 2026-11-16 — Recommendation snapshot retries and exact curation replay

93b469c passed all seven CI jobs 43773765716 and both CodeQL analyses 43773765754. Added bounded retries only for serialization/deadlock conflicts in consistent preference/recommendation reads, with identity/verification re-resolution for every attempt and cancellation preserved. A real PostgreSQL lock-coordination test forces a concurrent session-row update, proving each read recovers from the resulting snapshot conflict.

Normalized validated curation values/confidence to four-decimal storage before replay comparison, without mutating caller records. Extended database replay coverage with binary representation noise. Recommendation failures now use their own fixed error category rather than a release-operation category. No external calls, production data or deployments. Exact-commit CI pending.

## 2026-11-16 — Dedicated append-only curator role

9f74636 passed all seven CI jobs 43774180551 and both CodeQL analyses 43774180537. Added separate curator-role provisioning with public SELECT/observation INSERT only. feature-import now uses a bounded connection with fixed search_path and rejects elevated/owner/member/private-data roles before applying input. It cannot use migration credentials as a shortcut.

Added disposable role tests proving successful curation append, rejection of private reads/observation update/delete/taxonomy mutations and rejection after a private-column grant. No production role or corpus activated. Exact-commit CI pending; shell toolchains unavailable.

## 2026-11-16 — Inspectable owner-only preference signals

95eac9c passed all seven CI jobs 43774528051 and both CodeQL analyses 43774528063. Recommendation responses now include a bounded, key-ordered profile separating declared and rating-derived signals, using the same owner snapshot and eligible non-spoiler evidence as ranking. Inferred signals remain ephemeral; no rated-title lists, raw references or private record identifiers are returned. The dashboard labels derived direction/strength and explains its algorithmic meaning.

Extended PostgreSQL tests for cold start, declared/rating-derived transitions and foreign-profile isolation, plus mocked-browser derived display and late-account-result exclusion. Updated stale README/roadmap/context boundaries. No external AI, personal corpus, new grant or deployment. Existing client-component conventions apply; no dependencies/framework APIs changed. Exact-commit CI pending; no local Go/Node toolchains available.

## 2026-11-16 — Release history beyond the first hundred notices

92f4b4f passed all seven CI jobs 43775006044 and both CodeQL analyses 43775005934. Added keyset pagination for owned materialized in-app notifications, ordered by occurrence time and UUID. First-page polling retains bounded materialization; history reads do not generate new notices. Every page reauthenticates/reverifies, applies explicit ownership and existing forced RLS. Cursor boundaries cannot select another account. No schema/grant changes.

Dashboard history navigation pauses automatic refresh until returning to the latest page, preserves history during mark-read and retains the page after failures. Added cursor boundary unit tests, disposable PostgreSQL coverage for 225 equal-time notices/concurrent insert/read-state/foreign cursor isolation and HTTP validation, plus mocked-browser navigation/error retry/storage reset coverage. Existing framework pattern only; no dependencies changed. Exact-commit CI pending; no local toolchains or deployment.

## 2026-11-16 — Verified catalog and taste/release checkpoint

e661333 passed all seven CI jobs 43775283853 and both CodeQL analyses 43775283821, including cursor unit/database ownership tests, mocked history navigation and both real PostgreSQL browser suites. Prior implementation checkpoints 9057779, 93b469c, 9f74636, 95eac9c and 92f4b4f also passed their inspected CI/CodeQL runs recorded above. Repository metadata still reports public. No local test pass, deployment, provider activation or real-user data import is claimed.

Remaining work includes comprehensive approved metadata/availability coverage, real curated evidence and editorial review, cross-provider arbitration, durable per-title source scheduling, richer tracker/rewatches, background release delivery, recommendation retrieval/evaluation/hybrid AI and production security/operations. This documentation checkpoint records the verified implementation SHA; its own automatic checks run separately.

## 2026-11-16 — Per-title freshness and catalog field coverage

Inspected baseline a96f067: CI 43775494240 and Security 43775494171 passed; the subsequent scheduled Security run 43805152447 also succeeded. Extended the read-only catalog-status snapshot with independent per-title age/future ranges and observed/unobserved counts. Fresh partial imports no longer conceal old title observations in diagnostics. Disabled source title ages remain visible separately from enabled-source stale status.

Added source/catalog field-presence counts for known totals (including zero), synopsis, synonyms, genres, tags, studios and relations. Presence is explicitly not completeness, correctness, resolved relationships or availability. Added PostgreSQL partial-feed regression with equal-threshold, future-clock, disabled-source, sparse-field and known-zero cases. No new privileges, source activation or private-data queries. Exact-commit CI pending; local Go/Node toolchains unavailable.

## 2026-11-16 — Named bidirectional season continuity navigation

0fcc968 passed all seven CI jobs 43818782348 and both CodeQL analyses 43818782357.

Catalog details now resolve linked canonical title/media/status and derive only inverse PREQUEL/SEQUEL links. Single-sided provider assertions are navigable from either title; reciprocal assertions deduplicate. SIDE_STORY and other directed relations retain their explicit direction instead of acquiring invented chronology. This read-time public snapshot changes no stored provenance or private account data.

Extended public PostgreSQL/API tests for single-sided reversal, reciprocal deduplication, named linked media and directed non-continuity preservation. The dashboard labels links with title/type/status; a mocked-browser test follows a sequel then its prequel and verifies spoiler reveal resets. Read scoped web instructions; packaged Next guides are unavailable in the empty workspace, so rechecked the official use-client documentation and reused the existing component pattern without new framework APIs/dependencies. Exact-commit CI pending; local toolchains unavailable.

## 2026-11-16 — Historical confirmations stay out of offline release previews

fc02a4d passed all seven CI jobs 43819109502 and both CodeQL analyses 43819109407.

Review found the pure snapshot-pair release preview could report old backfilled confirmations as new events even though persistent ingestion baselines them. It now requires release times strictly newer than the supplied prior observation, using the same PostgreSQL microsecond precision, and rejects observation pairs indistinguishable at that precision.

Added boundary regressions for known/new titles, historical/equal/sub-microsecond/new confirmations, while preserving track identity, stable keys, schedule-only suppression and first-import baseline behavior. Documented the preview's limited two-snapshot state: it cannot reproduce persistent per-title history after intervening partial feeds. No production ingestion semantics, provider activation or delivery changed. Exact-commit CI pending; local toolchains unavailable.

## 2026-11-16 — Current provider title collections and localized aliases

fbea195 passed all seven CI jobs 43819431830 and both CodeQL analyses 43819431835.

Primary Jikan OpenAPI review confirmed titles is the current collection and legacy title/name fields are deprecated. The offline converter now handles modern-only title responses, prefers current Default/Japanese entries, retains localized and legacy alternatives as bounded deduplicated search aliases, and uses deterministic English/Japanese canonical fallbacks when needed. Ambiguous typed primary names and missing classified/legacy identity fail closed.

Added synthetic modern-only, mixed-field precedence/retention, future locale, duplicate/blank alias, fallback, conflicting-primary and malformed-value tests. Existing legacy fixtures remain required. No catalog API calls, live Jikan collection, personal data or source approval added. Exact-commit CI pending; local toolchains unavailable.

## 2026-11-16 — Verified metadata/continuity/release-preview checkpoint

6b05402 passed all seven CI jobs 43819766803 and both CodeQL analyses 43819766974. Prior commits 0fcc968 (per-title freshness/coverage), fc02a4d (season navigation) and fbea195 (historical preview correction) also passed their inspected CI and CodeQL runs recorded above. This includes disposable PostgreSQL public catalog tests, mocked named-season navigation, both real browser suites, recovery, backend and monitoring. Repository metadata still reports public.

No local toolchain pass, provider activation, personal data import or deployment claimed. Source coverage, field arbitration, curation/retractions, rewatches, background alert delivery, recommendation evaluation/hybrid AI and production security remain open. This documentation checkpoint records the verified implementation SHA; its own automatic checks run separately.

## 2026-11-16 — Atomic completed-pass archives and conditional rewatches

Baseline 7666a22 passed all seven CI jobs 43820146426 and both CodeQL analyses 43820146467. Added migration 000015 with an existing-entry cycle counter and forced-RLS completed-pass snapshots. Verified-owner conditional start requires the expected current cycle and COMPLETED status, snapshots count/exact progress, catalog total and rating, then resets progress/increments the pass atomically. Ratings/follows/notes remain; replay/concurrent stale starts return 409 without another reset.

Added PostgreSQL tests for exact snapshot retention, stale/incomplete/foreign starts, two-session races, injected rollback, immutable past ratings, bounded history, export and removal. Extended nonempty export/account-deletion fixtures, runtime/readiness/curator-worker privacy checks, recovery RLS/ledger and migration apply/rollback checks. Archived rows have SELECT/INSERT only through runtime grants. Local JSON entries report pass one but have no rewatch API. Dashboard controls remain pending. No personal data/source activation/deployment; local toolchains unavailable, exact-commit CI pending.

## 2026-11-16 — Rewatch dashboard and expected-pass edit guards

102ae78 passed all seven CI jobs 43824168647 and both CodeQL analyses 43824168605. Added verified-account viewing history, explicit archive/reset confirmation, error retry, older-pass navigation and current-pass display. Card remounts clear episode drafts on pass changes; component/session revisions fence late account results and browser storage remains unused.

Review found stale edits could cross a rewatch transition. Added optional expectedCycle to tracker PATCH/exact-episode PUT, with a shared conflict boundary and owner/row-lock recheck. Current dashboard edits opt in; legacy omission remains unconditional. Episode reads now lock the owned tracker before fetching progress, avoiding a joined pre-lock snapshot. Series ratings remain title-wide.

Added HTTP regressions for stale/current count/status/exact edits, two mocked browser tests and real PostgreSQL/browser coverage for archive/reset, rating/exact-history retention, foreign/replayed/stale requests and account reset. Scoped Next instructions read; packaged docs/toolchains unavailable, official use-client guide rechecked and existing patterns reused without dependencies. No source activation/deployment; exact-commit CI pending.

## 2026-11-16 — Conditional private tracker notes

17637cb passed all seven CI jobs 43824995507 and both CodeQL analyses 43824995477. Added GET/PUT private notes on existing user_anime storage. Verified-owner predicates/RLS and tracker locks apply; required timestamp comparison rejects stale saves, including across rewatches. Notes are bounded to 2000 Unicode code points, strict JSON/plain text, included in existing export and deletion cascades, and retained across rewatches. No schema/grant changes.

Added PostgreSQL/HTTP ownership, Unicode, conflict, export/clear/removal and rewatch retention tests; dashboard explicit edit/save/cancel/discard-reload with error draft preservation; two mock-browser privacy/conflict tests and real PostgreSQL browser save/foreign-read coverage. No browser persistence, external-model input, personal fixture data or deployment. Existing client-component instructions/patterns apply; local toolchains unavailable, exact-commit CI pending.

## 2026-11-16 — Preserve provider theme and demographic categories

0137585 passed all seven CI jobs 43825658289 and both CodeQL analyses 43825658280.

The neutral feed, offline Jikan converter, public details/API and coverage diagnostics now preserve themes and demographics separately instead of only flattening them into generic tags. Prior combined Jikan tags remain compatible; fields are label-presence facts, never inferred personal attributes, automatic style features or confirmed availability.

Added feed validation, converter category mapping, PostgreSQL details/coverage, local empty-array and mocked spoiler-display coverage. The dashboard shows demographics as sourced metadata, gates themes with synopsis/tags and avoids duplicate generic labels. Old snapshots/sparse providers retain empty categories. No schema/grant or source activation, no external AI/deployment; exact-commit CI pending and local toolchains unavailable.

## 2026-11-16 — Owner-scoped followed schedule calendar

e179d44 passed all seven CI jobs 43826058492 and both CodeQL analyses 43826058352. Added verified-owner calendar projection over follows/franchise scope, track preferences and latest schedule revisions. Window filtering occurs after latest-revision selection, preventing obsolete reschedules from reappearing. Confirmed releases are excluded; dates remain explicitly unconfirmed. Results cap at 100 with truncation, 1–31 UTC days and a two-second statement timeout.

Shared the existing bounded verified-snapshot retry helper with recommendation reads. Added PostgreSQL tests for partial revisions, exact boundaries, confirmed-release exclusion, overlapping/transitive follow deduplication, track/owner separation and result/range caps, plus HTTP validation. Added dashboard date controls/source labels, two mocked browser tests and real PostgreSQL/browser empty-state coverage. No schema/grants, alert materialization, provider activation or deployment; local toolchains unavailable and exact-commit CI pending.

## 2026-11-16 — Broader explicit CC0 anime-work classes

f18b60c passed all seven CI jobs 43827161580 and both CodeQL analyses 43827161582. Rechecked primary Wikidata ontology pages for anime film, original video/net animation and anime TV seasons. The bounded fixed-endpoint adapter now maps those directly asserted classes as MOVIE/OVA/ONA/TV while retaining entity identity and existing conservative factual fields.

Added synthetic transport/classification tests for each new class, format specificity over broad film, recognized-type conflicts, deprecated/preferred ranks and continued unknown status/count/schedules/relations. No subclass crawl, automatic parent/season links, availability, synopsis/media, inferred movie episode totals, larger request budget or actual source activation. Local toolchains unavailable; exact-commit CI pending.

## 2026-11-16 — Verified viewing-pass, notes, calendar and metadata checkpoint

56b7779 passed all seven CI jobs 43827868367 and both CodeQL analyses 43827868319. Every predecessor implementation in this continuation also passed inspected exact-commit CI and CodeQL: 102ae78 (watch-cycle API/migration), 17637cb (dashboard and stale-pass guards), 0137585 (private notes), e179d44 (theme/demographic categories) and f18b60c (follow calendar). Current tests include disposable PostgreSQL migration/rollback and fourteen-table forced RLS, nine-resource nonempty recovery/export, exact archive/replay/race/rollback, notes conflicts, latest-schedule windows and real browser account isolation. Source metadata still reports public.

The execution environment/toolchains are unavailable locally; no local pass claimed. No production source/service/model activated, deployment performed or personal fixture data committed. Complete licensed availability coverage, field arbitration, real reviewed evidence, finer season conventions, background delivery, hybrid AI/evaluation and production security/operations remain open. This documentation records the verified implementation SHA; its own automatic checks run separately.

## 2026-11-16 — Reviewed exact provider identity linking

Baseline 7e634dd passed all seven CI jobs 43828427356 and both CodeQL analyses 43828427554. Added bounded strict review files, a default-rollback catalog-map CLI and separate restricted mapper role. The shared catalog lock serializes exact links and source_records review audits with existing importers. Existing mappings cannot be changed in either unique direction; all batch effects roll back on conflict or missing target. Normalized per-link hashes make audit replay idempotent.

Added decoder/validation tests and PostgreSQL preview/replay/conflict/atomicity/audit tests. End-to-end import reuses a mapped canonical ID; a second provider link still cannot transfer metadata authority. Role tests reject private reads/grants, identity rewrites, source enabling, audit payload reads and metadata/deletion rights. No schema/private-data/source activation/deployment changes; local toolchains unavailable and exact-commit CI pending.

## 2026-11-16 — Public exact-ID lookup and identity display

ce3b460 passed all seven CI jobs 43836212263 and both CodeQL analyses 43836212323. Added exact case-sensitive GET /anime/lookup and public provider/externalId arrays in the existing consistent catalog-details snapshot. The dashboard renders these as text; metadata authority remains separate. External URLs, source review payloads/references and private records stay excluded.

Added narrow runtime column reads with readiness/grant fixtures and tests covering lookup validation, route precedence, canonical reuse, cross-provider authority retention, missing/local identities, URL-read denial and browser ID display. No migration or public catalog write route. Scoped web instructions read; packaged guides/toolchains unavailable, official use-client guide rechecked and existing component pattern reused. Exact-commit CI pending; no source activation or deployment.

## 2026-11-16 — Fail closed on indirect catalog-role data access

cf3b325 passed all seven CI jobs 43837173422 and both CodeQL analyses 43837173610. Review found the common writer/curator/mapper guard relied on fixed private table names, which could miss future tables, other schemas, owner-backed views and executable security-definer routines. It now permits relation privileges only on the reviewed non-RLS public catalog table contract and rejects indirect user-schema access. The check reads privilege metadata only.

Extended restricted mapper tests with a future forced-RLS table, a private view, another schema and PUBLIC-executable SECURITY DEFINER function; each is rejected while granted, and accepted again after unsafe execution/data privileges are revoked. Existing writer/curator suites remain required. No actual database privileges, private-data queries, schema migrations or deployment changed; exact-commit CI pending, local toolchains unavailable.

## 2026-11-16 — Verified canonical identity integration checkpoint

fa7f575 passed all seven CI jobs 43838337320 and both CodeQL analyses 43838337446. Predecessor ce3b460 passed CI 43836212263/Security 43836212323; cf3b325 passed CI 43837173422/Security 43837173610. Inspected the exact job results, including database identity review/audit/role regressions, both real-browser suites, frontend, backend, monitoring and recovery. Public repository state rechecked.

This introduces no schema migration or active source/deployment. Existing API installations need refreshed narrow runtime identity read grants. Local toolchains unavailable; no local pass claimed. Field arbitration, canonical merges/retractions, comprehensive licensed data/availability and production security remain separate work. This document records the verified implementation SHA; its own automatic CI runs separately.

## 2026-11-16 — Transactional catalog preview and opt-in metadata archives

Baseline 2d4ec30 passed CI 43839078696 and Security 43839078680. Added ImportWithOptions while preserving default Import/worker behavior, plus CLI -preview/-archive. Preview validates the complete persisted import then rolls back. Explicit archive stores selected neutral observations with type-scoped hashes in existing source_records, atomically with identities, metadata, schedules, events and high-water state. It requires separate historical-retention rights review; no raw envelope or private data is copied.

Added PostgreSQL tests for no-change defaults, complete preview rollback, original/revised observation retention, stale replay, late archive failure rolling back earlier facts/archives/new IDs/events, and API payload-read denial. Connection and rollback cleanup are bounded. No schema/API grants, source activation or deployment; local toolchains unavailable, exact-commit CI pending.

- Verified `2e0838b2b451f775d3823035014b902ebe62294f`: all seven CI jobs succeeded in run [43841704241](https://github.com/khordestan/anime-intelligence/actions/runs/43841704241); both CodeQL analyses succeeded in [43841704210](https://github.com/khordestan/anime-intelligence/actions/runs/43841704210), dependency review skipped on push.

### Bounded operator observation-history summary
- Added a read-only exact-provider/title history command with cursor pagination, cross-page field comparisons, first retained baseline semantics, fixed errors and payload/time/row bounds.
- Added a narrow operator reader SQL role and the existing fail-closed catalog privilege guard; API and mapping roles retain no payload access.
- Synthetic unit/database coverage checks field comparisons (including unknown versus known zero), timestamps, cursor boundaries, audit/provider exclusion, malformed archives and no private/write grants.
- Summaries expose only timestamps and allowlisted field names, never stored titles/prose or identifiers. No migration, service/source activation, raw-response archive, pruning or deployment.
- Go/Node remain unavailable locally; exact code-commit CI/CodeQL validation pending.

- Verified `a8a6ff0340cfea1c08a913e5479689a0faca2e1e`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/43842253400) succeeded, including the PostgreSQL archive/preview atomicity and cursor/diff/reader isolation tests. Both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/43842253442) succeeded; dependency review skipped on push.
- Local Go/Node executables remain unavailable; these are exact-commit GitHub Actions results, not local test claims. Public repository visibility was rechecked. No deployment, live source activation, personal data or credentials were introduced.


## 2026-11-16 — Bounded public continuity browsing

Baseline 9031a08 passed all seven CI jobs in [43842801409](https://github.com/khordestan/anime-intelligence/actions/runs/43842801409) and both CodeQL analyses in [43842801414](https://github.com/khordestan/anime-intelligence/actions/runs/43842801414). Rechecked current source, roadmap, recent commits and workflow results.

Added GET /anime/{id}/continuity with a public-only repeatable-read snapshot, bounded breadth-first expansion, deterministic fifty-title capacity/overflow detection, normalized reciprocal links and sorted provider labels. Nodes preserve known-zero/unknown totals and selected metadata observation times without synopsis/private data. SQL/overall deadlines bound reads; local mode does not invent edges. No migration/grant/follow/alert behavior changes.

Added synthetic PostgreSQL/HTTP tests for incoming/transitive links, reciprocity/provenance, unrelated relation exclusion, cancellation/validation, exact capacity versus overflow, dense graphs and defensive cyclic-data termination. Dashboard explicitly loads the view, warns on partial components, shows source labels and browses to related details with close/navigation request fencing. New mocked browser tests cover navigation, limits, errors/retry and late closed results; a real PostgreSQL/browser test covers anonymous access.

Scoped web instructions read; packaged Next docs and Go/Node toolchains are absent locally. Rechecked the official use-client guide and followed existing client-component patterns. No local test pass claimed; exact-commit CI/CodeQL validation pending. No source/model activation, personal fixtures or deployment.


- Verified aaee00cfe0187ae6346d794d8d12083cbb69b6eb: all seven CI jobs succeeded in [43844544929](https://github.com/khordestan/anime-intelligence/actions/runs/43844544929), including exact-capacity/dense-graph tests, both new mocked browsing tests and the anonymous real PostgreSQL browser test. Both CodeQL analyses succeeded in [43844544976](https://github.com/khordestan/anime-intelligence/actions/runs/43844544976); dependency review skipped on push.

## 2026-11-16 — Owner-scoped release catch-up visibility

Latest-page polling now selects at most 101 distinct eligible events, materializes at most 100, and reports created/hasMore from the same statement/transaction. The existing verified-owner context, account/advisory lock ordering, track/subscription filters and uniqueness boundary remain in effect. History pages omit status and cannot drain the pending backlog.

Added synthetic database/HTTP coverage for 101 eligible events, exact 100/1 draining, overlapping follow paths, ineligible track/old/unrelated exclusion, foreign-account isolation, history non-materialization and a newly admitted 120-event track backlog. Dashboard catch-up state is separate from history cursors, survives failed page requests, clears on a successful checked batch and resets across accounts; mocked browser coverage added.

No schema/grant/source activation, new delivery channel, changed confirmation rule, personal data or deployment. Local toolchains remain absent; exact-commit CI/CodeQL pending.


## 2026-11-16 — Verified continuity and release catch-up checkpoint

8bcd38b1e52594633cc1478690669aa8080c06ca passed all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/43845176573) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/43845176686); dependency review skipped on push. Predecessor aaee00cfe0187ae6346d794d8d12083cbb69b6eb also passed all seven CI jobs 43844544929 and both CodeQL analyses 43844544976. Inspected exact job results and targeted database/browser logs.

Coverage includes two new public graph PostgreSQL/HTTP tests (direction/provenance/known totals and exact capacity/overflow/dense/cyclic data), the bounded materialization/owner/history/track backlog test, three new mocked browser tests and an anonymous real PostgreSQL browser test. The existing backend race/vet/build/vulnerability, frontend unit/typecheck/build/audit, both full-stack suites, migration/RLS/rollback, recovery and monitoring checks remain green.

Repository visibility remains public. No schema or role-grant update, real account data, credentials, live source/model activation or deployment. Local Go/Node are absent; verification occurred in GitHub Actions. Complete approved metadata/availability collection, durable per-title source scheduling, field corrections/arbitration, background channels, AI/evaluation and production security/operations remain pending. This checkpoint records the verified implementation SHA; its documentation-only automatic checks run separately.


## 2026-11-16 — Combined public catalog metadata filters

Baseline d255709 passed all seven CI jobs 43845684186 and both CodeQL analyses 43845684107. Rechecked current source, roadmap, recent commits and Actions results.

Added optional combined filters for source genre/theme/demographic/studio and scalar format/status/year/quarter. Exact case-insensitive labels and literal title/native/alias search combine before deterministic ordering/pagination; absent metadata cannot satisfy a filter and legacy tags do not imply themes. Strict bounded UTF-8/query/enum/duplicate validation and fixed errors apply. Public filtered PostgreSQL reads have statement/overall deadlines; JSON mode only uses its known starter facts. No schema/grants, provider/model activation, private-data reads or deployment.

Synthetic local/HTTP/PostgreSQL coverage checks combined filters, sparse/category semantics, aliases, literal SQL/wildcard-looking values, filter-before-pagination, bounds, cancellation and anonymous access. Local Go/Node remain unavailable; exact-commit CI/CodeQL pending.


- Verified f3066abeb4e3d3ffa2c7583ae510974191553bb2: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/43847016587) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/43847016569) succeeded; dependency review skipped on push.

## 2026-11-16 — Explicit dashboard metadata filters

Added an apply/reset form for the combined scalar and sourced-label filters, with byte/year bounds, loading/empty/retry states and explicit sparse-metadata/quarter semantics. Search works without q, keeps alias searches when supplied, withholds unfiltered fallbacks for active filters and fences stale results by request/account revisions. Tracker mutations and account switches reset filters with the query. Show more expands eight suggestions at a time within the current 100-result request; the cap is labeled.

Added a safe allowlisted filter-validation client error, a corresponding transport test, mocked browser coverage for combined URL encoding/draft application/clear/expansion and late-result/failure retry, plus a real anonymous PostgreSQL/browser filter test. Scoped web instructions read; packaged guides and local Go/Node remain absent, official use-client guide rechecked. Exact UI-commit CI/CodeQL pending. No schema/grants, source activation, personal fixtures, model calls or deployment.


## 2026-11-16 — Verified combined metadata filtering checkpoint

cbdc71ddbeb3a32664c3f7022319e419aee8a71d passed all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/43847834579) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/43847834679); dependency review skipped on push. Its API predecessor f3066abeb4e3d3ffa2c7583ae510974191553bb2 passed all seven CI jobs 43847016587 and both CodeQL analyses 43847016569. Inspected exact job results and targeted PostgreSQL/browser logs.

Coverage includes normalization/validation, sparse/category distinctions, combined labels/scalars, aliases, literal injection/wildcard-looking labels, filter-before-pagination, cancellation, draft/apply/reset/expansion, stale response replacement, retry and real anonymous PostgreSQL filtering. Existing authorization/RLS, recovery, monitoring, backend race/vet/build/vulnerability and frontend unit/typecheck/build/audit suites remain green.

Public visibility rechecked. No schema/grants, private-data scope expansion, real fixture data, source/model activation or deployment. Local Go/Node are absent; these results are GitHub Actions verification. Comprehensive licensed metadata/availability, facets/broader catalog pagination, durable source jobs, corrections/arbitration, background delivery, AI/evaluation and production operations/security remain pending. This checkpoint records the verified implementation SHA; its own documentation checks run separately.

## 2026-11-16 — Bounded source-label discovery

Added public genre/theme/demographic/studio facets with case-folded deduplication, literal prefix matching, strict validation, lookahead and SQL/context deadlines. Synthetic PostgreSQL/HTTP tests cover categories, sparse fields, exact/overflow caps and literal prefixes. No grants/schema/private-data/source activation/deployment. Local toolchains unavailable; exact commit CI pending. Baseline 199d6e8 passed CI 43863761962 and Security 43863762039.

## 2026-11-16 — Explicit dashboard source-label suggestions

Connected source-label discovery to filter drafts with manual requests, theme spoiler warning, empty/truncated/error states and edit/unmount response fencing. Selection does not apply filters automatically. Added mocked browser coverage for explicit loading, draft-only selection, theme intent and clearing. Scoped web instructions read; packaged guides/toolchains absent, official client guide rechecked. No local test pass; exact CI pending. No provider activation, private-data changes or deployment. Baseline 8b4f9b8 passed CI 43865023549 and Security 43865023553.

## 2026-11-16 — Distinguish discovery-prefix and filter-value bounds

Added local 128-byte UTF-8 discovery-prefix validation with a specific message and stale-request invalidation; complete filter values still permit 256 bytes. Extended browser coverage with a multi-byte 129-byte prefix and zero-network assertion, followed by valid recovery. Baseline ee618735 passed CI 43866035260 and Security 43866035145. Local toolchains absent, official client guide rechecked; exact follow-up CI pending. No schema/grants/source activation/deployment.

## 2026-11-16 — Local catalog read-boundary parity

Direct Local.Catalog previously bypassed filtering validation and could pass negative offsets into slice operations. It now delegates to FilterCatalog, matching PostgreSQL repository bounds; local filters also honor cancellation before sparse early returns. Added direct invalid-limit/offset, cancellation, sparse cancellation and ordinary-search regressions. No schema/grants/source activation/private-data/deployment changes. Local toolchains unavailable; exact CI pending.

## 2026-11-16 — Bounded catalog offset navigation hints

Added optional nextOffset/previousOffset response fields without an extra count query. Full-page hints are explicitly not existence guarantees or stable snapshots; prior offsets clamp to zero and successors stay within the existing cap. Synthetic HTTP coverage covers full/short/empty-filtered pages and boundary behavior. No schema/grants/private-data/source activation/deployment changes. Local toolchains unavailable; exact CI pending. Baseline d8b1811 passed all CI 43870396211 and Security 43870396248 jobs.

## 2026-11-16 — Dashboard catalog page navigation

Connected bounded next/previous offset hints to dashboard search with query/filter preservation, per-page expansion reset, retained results on failures and exact-destination retry. Existing request/account revisions reject late pages. Added browser failure/retry/return-navigation coverage with encoded filter assertions. Scoped web instructions read; packaged guides/toolchains absent, official client guide rechecked. No schema/grants/source activation/deployment. Exact CI pending.

## 2026-11-16 — Reset unapplied catalog drafts across account boundaries

Review found that unchanged applied-filter props could leave child draft text mounted across sign-in/sign-out. Keyed the form by account/public identity so draft state and pending label request lifetimes reset together. Added anonymous-to-account-to-anonymous browser regression coverage. Scoped web instructions read; packaged docs/toolchains absent, official client guide rechecked. No local pass claimed; exact CI pending. No schema/grants/source activation/deployment.

## 2026-11-16 — Validate retained catalog history facts

History decoding now checks canonical UUID syntax and runs neutral-feed validation at the retained observation time, rejecting corrupt status/count/release/relation facts without emitting raw values. Updated valid synthetic envelope fixtures and added invalid-ID/status/title/future-release/self-link cases. No schema/grants/source activation/deployment. Baseline f856a827 CI and Security green; local toolchains unavailable, exact CI pending.

## 2026-11-16 — Aggregate catalog Prometheus diagnostics

Added opt-in catalog-status -format=prometheus with fixed aggregate freshness/coverage/schedule gauges, snapshot timestamp, validation and no dynamic/private/provider labels. JSON default retained. Added unit contract/privacy/invalid-snapshot tests and a synthetic real-renderer promtool check to monitoring CI. No schema/grants/source activation/collector/receiver/deployment. Local toolchains absent; exact CI pending. Baseline a369580 passed CI 43882243818 and Security 43882243978.

## 2026-11-16 — Verified catalog telemetry checkpoint

0b000bd4e4f5b6ffd8c951be4fbb5686d3c65169 passed all seven CI jobs in https://github.com/khordestan/anime-intelligence/actions/runs/43889783641 and both CodeQL analyses in https://github.com/khordestan/anime-intelligence/actions/runs/43889783434 (dependency review skipped on push). This includes real-renderer promtool parsing, backend race/vet/build/vulnerability, migrations/RLS/recovery and browser suites. Updated stale context pending notes; all included catalog behavior is covered by this exact checkpoint. Local toolchains unavailable; no local pass claimed. No source/model/collector/receiver or deployment activated. Documentation-only follow-up checks run separately.

## 2026-11-16 — Catalog diagnostic query bounds

Bound Inspect SQL statements to two seconds and rollback cleanup to five seconds; reject zero/pre-epoch clocks. Existing PostgreSQL diagnostic coverage verifies normal reads. No migration/grants/source activation/deployment. Local toolchains unavailable; exact CI pending.

## 2026-11-16 — Restrict catalog diagnostic command credentials

Applied the common fail-closed catalog privilege guard before diagnostic queries; elevated/account-capable roles are refused, matching other catalog operator commands. Pinned search_path, bounded connection setup/cleanup and validated format/window flags before connecting. Existing restricted catalog-role and diagnostics suites remain required. No new grants/schema/source activation/deployment. Local toolchains absent; exact CI pending. Baseline 6739ad59 passed CI 43891035559 and Security 43891035457.

## 2026-11-16 — Explicit catalog metadata deficit gauges

Added nine fixed missing_metadata gauges derived from canonical title counts minus field presence, including unobserved titles. Reject impossible coverage above observed-title totals before any output. Tests cover partial observations, unknown fields, empty catalogs and invalid coverage; existing monitoring fixture parses the expanded renderer output. No schema/grants/source activation/collector/deployment. Local toolchains unavailable; exact CI pending.

## 2026-11-17 — Inactive catalog freshness alert rules

Added optional catalog monitoring rules gated to explicit targets and healthy snapshot times. Synthetic promtool tests cover persistence/recovery, old/future data suppression, target isolation, down/missing distinction and unmarked targets. CI validates/tests the new rules; production configuration and receivers remain untouched. Baseline a7378c4 passed CI 43892390543 and Security 43892390349. Local toolchains unavailable; exact CI pending. No source/service/receiver/deployment activated.

## 2026-11-17 — Episode-count range discovery

Added inclusive optional minEpisodes/maxEpisodes filters (0–100000) in local/PostgreSQL search and dashboard inputs, with API duplicate/empty/integer validation and reversed-range rejection. Unknown totals cannot satisfy a range; known zero remains searchable. Added PostgreSQL sparse/zero/exact/range regressions. No monitoring work, schema/grants/source activation/deployment. Local toolchains unavailable; scoped web instructions and official client guide checked. Exact CI pending.

## 2026-11-17 — Catalog release-year ordering

Added TITLE/NEWEST/OLDEST sorting to public catalog API and dashboard controls. Year sorting keeps unknown years last and title/UUID tie-breakers stable, before pagination. Uses fixed parameterized SQL expressions; invalid sort values are rejected. Added PostgreSQL newest/oldest/tie/unknown/pagination tests. Scoped web instructions and official client guide checked; local toolchains absent, exact CI pending. Product feature only; no monitoring/schema/grants/source activation/deployment.

## 2026-11-17 — Catalog release-year ranges

Added minYear/maxYear to local/PostgreSQL search, strict API parsing and dashboard inputs. Inclusive bounds exclude unknown years and compose before sorting/pagination; exact-year filters still combine with AND. Added PostgreSQL boundary/unknown/exact/reversed cases and expanded combined dashboard request coverage. Local toolchains unavailable; scoped web instructions and official client guide checked, exact CI pending. Product work only, no monitoring/schema/grants/source activation/deployment.


## 2026-11-17 — Library browsing product controls

- Added independent library title search, rated/unrated/exact-score filtering and default/title/rating/watched-episode sorts, combined with watch status.
- Unrated entries remain last for both rating directions; ties use title/ID. Presentation sorting does not mutate the stored library. Clear restores server order and all filters; account changes clear the controls.
- Added matching counts, explicit empty states and pressed status buttons. Catalog search remains independent.
- Playwright covers combined filters, sort ties, unrated behavior, clear/refresh and absence of writes; extended account-switch coverage checks resets.
- Prior checkpoint 7a35e5e: inspected all seven CI jobs successful (43976735338) and Security workflow successful (43976735440).
- No local executor/toolchain is available. Exact pushed-commit CI is pending. No monitoring changes, real user data, source activation or deployment.

Validation for `efa8be2aeb64ea85f4300d998d26710d04b85ccb`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/43987814257) passed, including backend unit/race/vet/build, frontend typecheck/build and 30 mocked browser tests, PostgreSQL integration/migrations/rollback, recovery and both real full-stack browser suites. Both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/43987814284) passed; dependency review skipped on push as configured. No local tests were run. This follow-up records those exact-commit results without changing product code.

## 2026-11-17 — Library progress-mode browsing

- Added a current-pass exact-selection/count-mode filter alongside existing library controls, without changing tracker data or browser storage. Absent flags preserve count-mode compatibility.
- Extended browser coverage for combined rating/status filtering, refresh preservation, clear/default-order restoration and account-switch reset; no-write assertions retained.
- Inspected baseline `6548649996a72e5164c7cde9e2814c8e98ab0fdd`: CI and Security push workflows completed successfully. Scoped web instructions and current official client-component guide read; installed guide/local Go/Node toolchains are unavailable. New exact-commit CI pending.
- No schema/grant changes, source activation, real user data or deployment.

Validation for `de4172a0d6ab64f802db4c819b1e10b6f29bd852`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44046690497) passed, including the extended mocked-browser and account-switch tests, both PostgreSQL browser suites, database integration/migrations, backend, recovery and monitoring. Both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44046690353) passed. Local tests were unavailable. This follow-up records exact-commit hosted results only.

## 2026-11-17 — Durable per-title Wikidata ingestion

- Migration 000016 adds explicit public-ID jobs, due times, per-title refresh intervals, five-claim budgets and digest-fenced expiring leases. API roles receive no grants; the dedicated catalog-writer role receives narrowly scoped queue rights.
- The opt-in worker now schedules configured IDs and processes one due title per minute poll, preserving provider-wide cooldowns and separate per-title refresh targets. Removed IDs stay dormant. Exhausted jobs require operator review; adding/configuring IDs does not bypass provider Retry-After.
- Extracted the existing import transaction body so facts/provenance/schedules/events/high-water mark and a live lease acknowledgement commit together. Failed imports or late acknowledgements roll everything back. No row transaction spans HTTP; cancellation attempts bounded failure cleanup.
- Added synthetic PostgreSQL tests for approval checks, no locks during fetch, restart durability, configured-ID scope, retry exhaustion/poison isolation, provider cooldowns across new IDs/restarts, lease expiry/replacement, revocation during fetch, wrong response IDs, cancellation and late acknowledgement rollback. Added queue polling tests and updated migration apply/rollback CI and recovery ledger expectations.
- Baseline 211165ab85e787ee7163448aa3044167e3ee007e CI and Security workflows were inspected and passed. Local Go/Node toolchains are unavailable; Docker socket access is denied. New exact-commit hosted CI is pending, not a local pass.
- No live source/host activation, deployment, secrets, account data or real watch history. Existing installations must apply migration 000016 and reapply catalog-writer grants before running the new worker.

Validation for `368e89685ff894e9861e37eed564b059de600c18`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44049732218) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44049732210) passed. Inspected database job logs: all three new queue test groups and their subtests ran and passed, including no row locks during HTTP, concurrent collector exclusion, restart/per-title scheduling, terminal retry budgets, source cooldown, stale leases, approval revocation, wrong IDs, late acknowledgement rollback and cancellation cleanup. Backend unit/race/vet/build/vulnerability checks, migration apply/rollback, recovery, frontend and both real-browser suites passed. No local tests were run. This follow-up records verified implementation results; no product behavior changes.

## 2026-11-17 — Reviewed TVmaze season integration

- Reviewed TVmaze's primary API/copyright and linked CC BY-SA 4.0 terms. Added a bounded fixed-endpoint HTTPS adapter for explicitly reviewed anime show/season pairs; animation alone is insufficient.
- Retains source season identity, adapted display label, explicit premiere year and regular season-local schedules. Unknown totals/status/genres remain unknown; excludes specials without renumbering and never turns air dates into release confirmations.
- Added provider selection to catalog-fetch and the durable worker. Fixed provider/ID allowlists, independent approval/high-water/job/cooldown state, existing lease fences/retry budgets and atomic import acknowledgement are preserved. TVmaze review opt-in is mandatory. Migration 000017 expands bounded job IDs, with atomic refusal to downgrade while TVmaze work remains.
- Dashboard attribution and adaptation/license links cover details and other views. Source notices and downstream distribution scope are documented; no source rows or host processes are enabled.
- Synthetic tests cover membership, typed/oversized/malformed data, redirects, pacing/cooldowns, privacy-safe errors, nullable/special/gapped episodes, classification gating, provider isolation/spoofing, schedule-only imports, retained downgrade work, and browser credit links.
- Baseline ae799ae8af44c81cec8cfd9d8d82d7cb057eb778 CI and Security workflows passed on inspection. Local Go/Node toolchains and installed Next guide are unavailable; current official client-component guide checked. Exact new CI pending.
- No real accounts/watch histories, provider catalog payloads, secrets, collection, source activation or deployment.

Validation for `fa1e4065008a4e0e4a41afaae3fca593ccb1f009`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44052581312) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44052581246) passed. Inspected logs confirm TVmaze provider unit/race checks passed, all 31 mocked browser tests passed, and both TVmaze PostgreSQL test groups ran and passed (source isolation/cooldowns/unconfirmed schedules and retained-work migration rollback). Existing integration/migration/recovery and real-browser suites also passed. No local tests or live provider catalog requests were performed. This checkpoint records verified results and clarifies operator documentation/CLI help.

## 2026-11-17 — Reviewed secondary episode-source imports

- Added explicit schedules-only imports for existing exact provider mappings, preserving all canonical metadata/provenance/relations. Independent per-title observation baselines suppress historical confirmations; known-total and existing episode-authority conflicts roll the entire batch back.
- Added pinned durable job scopes, pre-fetch mapping checks, in-flight canonical mapping and scope fences, transactional compatibility markers and an old-binary database write guard. Queued acknowledgement remains atomic with facts and observations. Full metadata updates cannot invalidate retained foreign-source episode counts.
- Added per-episode provider provenance and mixed-source TVmaze attribution in API/dashboard details. Migration 000018 adds public-only observation timestamps and the scope protocol; catalog-writer grant refresh required, no API grants added. Downgrade refuses retained scoped jobs/observations.
- Synthetic PostgreSQL coverage checks previews, untouched metadata/relations, first/title-local baselines, correction/deduplication, late batch rollback, known-zero/total/authority conflicts, required mappings, scope mismatch/legacy guards, stale mapping and late acknowledgement rollback, role boundaries and downgrade preservation. Browser coverage checks primary-metadata/TVmaze-schedule attribution.
- Baseline e656702c083ad88c1e6565273e59b719ef13e0a6 CI and Security runs were inspected and passed. Scoped web guidance/current official client guide checked; local Go/Node/installed guides unavailable. Exact new hosted CI pending.
- Provider research found the original manami dataset archived; no aggregate dataset was adopted or downloaded. No real provider content, private watch history, secrets, source/host activation or deployment.

Validation for `9aa30dab242cee2d69ff4ab67caf05b996bc213f`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44062052030) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44062051958) passed. Database logs confirm all four new schedule-scope groups executed and passed: metadata/per-title baseline preservation, atomic conflict rollback, mode/mapping/acknowledgement fences and retained-work downgrade. Frontend logs confirm all 32 mocked browser tests passed, including both TVmaze-only and mixed-source attribution cases. Backend unit/race/vet/build/vulnerability checks, migration apply/rollback, recovery and both real PostgreSQL/browser suites passed. No local tests were run. This follow-up records verified implementation results only.

## 2026-11-17 — Reviewed label-field source choices

- Added explicit SELECT/BASE decisions for six label fields, with approved exact identities, CAS revisions, current-decision replay, retained audit history, bounded strict manifests and default rollback-only CLI previews.
- Primary imports atomically retain their latest unprojected facts and reapply selected fields. BASE restoration uses the latest primary value. Existing archives retain original primary facts; filters/aliases/facets/details read the same resolved projection.
- Added a dedicated field-curator role without canonical-title/schedule/source-approval/account rights; factual workers cannot write decisions. API reads only public provenance columns. Startup/readiness now checks that narrow schema/grant contract.
- Added a projection compatibility marker/old-binary guard and retained-decision downgrade refusal. Source-presence diagnostics read primary base facts rather than miscrediting secondary labels.
- Added validation, preview/replay/refresh/restoration/empty-value/query consistency, CAS concurrency, batch/audit rollback, role boundary and downgrade tests; dashboard field provenance preserves spoiler boundaries.
- Baseline cc3628c0739a53319e68a9d30342bf52ec70852f CI/Security workflows were inspected and passed. Local toolchains/installed web guide unavailable; scoped web/current official client guide checked. Exact new hosted CI pending.
- No actual source collection, real metadata corpus, private watch history, credentials, source/host activation or deployment.

Follow-up review: the curator now revalidates the retained primary neutral title before projection. A regression removes its required external ID from otherwise valid JSON and verifies the decision/audit/projection transaction rolls back. Baseline implementation `bb8434577f750f628864ae85fc163a9e50159236` passed all seven CI jobs (44073033687) and both CodeQL analyses (44073033693); this follow-up requires its own CI.

Validation for `ce6ec443d39e441c74e85166987958bf41930c01`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44073596242) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44073596425) passed. Database logs confirm the three field-choice groups ran and passed, including refresh/restoration/query consistency, CAS concurrency, audit/batch/corrupt-base rollback, role restrictions, projection compatibility and retained-decision downgrade. The implementation run passed all 33 mocked browser tests; the follow-up frontend and both real-browser suites passed too. Backend unit/race/vet/build/vulnerability, migrations and recovery checks passed. No local tests were run. This documentation-only follow-up records exact-commit validation.

## 2026-11-17 — Reviewed scalar metadata choices

- Extended strict SELECT/BASE manifests with typed releaseYear, season, mediaType and episodeCount scalars. Unknown and known-zero totals remain distinct; scalar representations normalize before decision hashing while label replay hashes remain unchanged.
- Scalar decisions update only the selected canonical column and resolved payload atomically. Primary imports resolve all active choices before canonical projection, preserve raw primary facts, and reject counts that invalidate incoming or retained episodes from any source. Conflicting BASE restores remain atomic.
- Migration 000020 expands field/value constraints and refuses downgrade while scalar decisions/audits remain. The field curator gains only four scalar/updated-at writes and public schedule reads; canonical names/prose/status, schedule writes, source approval and private accounts remain inaccessible.
- Added strict-type/unknown tests, preview/apply/query/filter/refresh/restore/replay checks, retained-history/late-batch/refresh rollback and role/downgrade regressions. Dashboard displays resolved format/year/quarter/count with friendly field provenance and unknown/zero browser coverage.
- Baseline 5f1dba2a97bd811291cb2fb3342a8df4f2a6c12b CI and Security workflows passed on inspection. Local Go/Node/installed guides unavailable; scoped web/current official client guide checked. Exact new hosted CI pending.
- No provider requests, real metadata corpus, secrets, personal watch history, source/host activation or deployment.

Validation for `df9ca7be8614bf631dbec94576b3ada23a599f3d`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44076966584) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44076966571) passed. Database logs confirm both new scalar groups executed and passed (preview/refresh/restoration/public filtering/unknown-zero/replay and retained-episode/batch/refresh/restore/role/downgrade safety). Frontend logs confirm all 36 mocked browser tests passed, including the three scalar unknown/zero/known cases. Backend unit/race/vet/build/vulnerability, migration apply/rollback, recovery and both real PostgreSQL/browser suites passed. No local tests were run. This follow-up records exact-commit implementation validation only.

## 2026-11-17 — Reviewed names, prose and display status

- Extended typed field reviews with title/nativeTitle/synopsis/status strings, bounded UTF-8/control validation, normalized replay, optional empty text, atomic canonical/resolved projection and latest-primary BASE restoration.
- Primary imports preserve selected names/prose/status while retaining raw observations. Season announcements compare the retained primary status, preventing display choices from inventing or suppressing source transitions. No curation events or episode/identity/private changes.
- Migration 000021 expands text constraints and refuses destructive downgrade of selected/restored history. Refreshed curator grants add only the four public columns; no new API grants.
- Added strict string/type/bound tests and real PostgreSQL preview/replay/search/status/provenance/refresh/restore/empty/late-audit rollback and primary-event regressions. Updated former scalar-only privilege tests for the explicitly expanded curator scope. Recovery ledger and explicit up/down CI include migration 000021.
- Baseline 3ebfc9e705111e68318af452342fa67b1eea50af CI 44077841455 and Security 44077841569 passed on inspection. Local Go/Node unavailable; new exact hosted checks pending.
- No real metadata, prose copying, personal data, provider calls, source/host activation or deployment.

Validation for `daac704276d539b9987310823357814ee726d716`: all seven [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/44079706907) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/44079706507) passed. Database logs confirm text preview/replay/search/provenance/refresh/restoration/empty/audit-rollback/downgrade and both announcement-baseline scenarios executed and passed. Existing 36 mocked browser tests, real PostgreSQL/identity browser suites, backend unit/race/vet/build/vulnerability, migration up/down and logical recovery checks passed. No local checks ran. This documentation-only follow-up records exact implementation validation.

## 2026-11-17 — Reviewed unused identity retirement

- Added strict exact-row retraction manifests and a preview-default catalog-retract command with dedicated credentials and the common public-only role guard.
- Dependency checks refuse primary/selected/retained-review facts, schedule observations/revisions, relation edges/claims, durable jobs and non-mapping archives. Disabled sources can still be corrected; no canonical/private/event data is moved or erased.
- Migration 000022 adds immutable reviewed tombstones and an invoker-rights, shared-lock identity trigger preventing mapping/import resurrection. Refreshed writer/mapper SELECT grants are required; API grants unchanged. Downgrade refuses retained retirements.
- Added strict manifest, preview, disabled-source/concurrent replay, lookup/details preservation, stale/different-review, mapping/import rollback, restricted-role, dependency matrix, batch rollback and downgrade regressions. Recovery ledger and explicit migration CI include 000022.
- Baseline a13073a24021094bfb7d5ff84f85f64d2cfecad2 CI 44080181072 and Security 44080181088 passed on inspection. Local Go/Node unavailable; exact implementation hosted checks pending.
- No real metadata, secrets, account/watch data, provider requests, source/host activation or deployment.

## 2026-11-24 — Offline catalog field presence

- Added opt-in snapshot field and per-track row counts to catalog-check. Scheduled timestamps and confirmed releases are counted independently, and known zero episode totals count as present.
- Local `make check` passed with VCS stamping disabled for the linked worktree. The CLI smoke test verified the unchanged default response and reported counts. Local PostgreSQL integration tests skipped without `TEST_DATABASE_URL`. Hosted checks pending.

## 2026-11-24 — Offline schedule-move preview

- Added opt-in snapshot-pair output for changes to two known unconfirmed times on the same title, episode and track. The preview distinguishes earlier and later moves and leaves release event output unchanged.
- Local `make check` passed. A synthetic CLI smoke test verified the optional report and unchanged default output. Hosted checks pending.

## 2026-11-24 — Offline Global Essentials baseline

- Added a deterministic public-recognition ranker and an offline, bounded fixture evaluator with a title-order baseline. The report includes evidence citations and measures missed held-out titles even when they lack qualifying evidence.
- Local Go tests, race tests, vet and build passed with `make check`. The documented CLI example returned the expected metrics. PostgreSQL integration tests skipped without `TEST_DATABASE_URL`. Hosted checks pending.

## 2026-11-24 — Opt-in Global Essentials lane

- The offline baseline from PR #42 passed [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45406330117) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45406329929) on `091b6f4` and merged.
- PostgreSQL startup now validates an optional reviewed manifest against catalog IDs and titles. The public API and dashboard display the bounded, evidence-linked snapshot.
- Local `make check`, frontend dependency install, unit tests, production build and typecheck passed. PostgreSQL integration tests skipped locally without `TEST_DATABASE_URL`. Hosted checks and the full local browser run are pending.

Validation for `d8b8b43` (PR #43): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45407707023) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45407706857) passed. All 45 local mocked browser tests passed, including signed-out Essentials access and the evidence link. The PR merged as `625ff78`.

## 2026-11-24 — Global Essentials evidence coverage

- Added candidate and relevant-title eligibility counts and recall ceiling at k. Evaluated five films using independent public award results and critics' poll inclusion. The single-source pilot reached its coverage ceiling and left representative evaluation open.
- Local `make check` passed with VCS stamping disabled for the linked worktree. The pilot reproduced the expected one eligible relevant title out of three. Local PostgreSQL integration tests skipped without `TEST_DATABASE_URL`. Hosted checks pending.

Validation for `8db4d94` (PR #44): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45408374539) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45408374397) passed. The PR merged as `b0bd258`.

## 2026-11-24 — Explore an evidence feature from Discovery

- Added canonical feature keys alongside the existing top-four Discovery feature names. Result cards use those keys to apply the existing filter and reset the paging cursor.
- Local `make check`, dependency install, frontend unit tests, build and typecheck passed. All five focused Personal Match and Discovery browser tests passed. PostgreSQL integration tests skipped locally without `TEST_DATABASE_URL`. Hosted checks pending.

Validation for Discovery feature navigation, `7846301` (PR #45): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45408956844) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45408956125) passed, including the PostgreSQL regression. The PR merged as `ee6c8ca`.

Validation for catalog snapshot coverage, `4a1cf1a` (PR #46): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45409247536) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45409247438) passed. The PR merged as `98e7327`.

## 2026-11-24 — Release backlog burst

- Added a synthetic PostgreSQL regression with sixteen simultaneous polls across two verified owners following the same title on distinct tracks. Each owner drains 230 events within the per-request batch cap, with exactly 230 stored notices and no cross-track rows.
- Local Go tests, race tests, vet and build passed. The focused PostgreSQL test passed three times under the race detector using a disposable PostgreSQL 17 container. Hosted checks pending.

Validation for `8af6bbf` (PR #47): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45409964608) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45409964239) passed. The database job ran and passed the concurrent catch-up test. The PR merged as `2ee0576`.

## 2026-11-24 — Paged dashboard follows

- The release dashboard reads 50 follows at a time and uses the verified caller's exact subscription lookup for off-page library titles. Follow navigation avoids refreshing or materializing notifications.
- Local `make check` passed. The focused owner-scoped PostgreSQL lookup and paging test passed under the race detector. Frontend unit tests, typecheck, build and all eight focused release browser tests passed. Hosted checks pending.

Validation for `81bc8b5` (PR #48): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45410810883) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45410811042) passed. The full local PostgreSQL runtime suite also passed under the race detector against a disposable PostgreSQL 17 instance. The PR merged as `7fff9d7`.

## 2026-11-24 — Catalog schedule revision history

- Added a bounded public schedule history API and an explicit detail-panel action. Each record preserves its release track, historical timestamps and source attribution.
- Local `make check` passed. The focused PostgreSQL API test passed under the race detector. Frontend unit tests, build, typecheck and all three focused schedule and attribution browser tests passed. Hosted checks pending.

Validation for `938a697` (PR #49): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45411595795) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45411595440) passed. The PR merged as `f117513`.

## 2026-11-24 — Confirmed movie notices

- Captured canonical format when recording a release event and labeled confirmed first movie releases in-app. The existing event and notification keys, track matching, subscription boundaries and baseline rules still apply.
- Local `make check` passed. Three focused PostgreSQL release tests passed under the race detector, covering movie confirmation versus elapsed schedules, subsequent format corrections, concurrent catch-up and owner isolation. Hosted checks pending.

Validation for `105f6d6` (PR #50): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45412162377) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45412162363) passed. The PR merged as `58dd57b`.

Validation for `41b85d1` (PR #51): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45412567346) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45412567580) passed. The PR merged as `34fff33`.

## 2026-11-24 — Opt-in in-app schedule-move notices

- Added explicit publication of reviewed offline schedule moves and per-follow owner consent. Consent does not replay earlier changes. Notices preserve source, track and prior/current unconfirmed schedule times. Migration 000024 defaults existing follows to off, and downgrade guards retained consent.
- Local `make check`, PostgreSQL consent/publication and rollback guard regression under the race detector, frontend unit tests, typecheck, build and eight focused browser tests passed. Hosted checks pending.

Validation for `8957fac` (PR #52): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45413876616) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45413876912) passed, including migration and recovery jobs. The PR merged as `4afffb8`.

## 2026-11-24 — Catalog evidence tool

- Added read-only sourced catalog evidence for bounded questions about one or two chosen titles. The deterministic selector only projects six public fields and preserves known zeroes and field-level source choices.
- Go checks, frontend unit tests, build, typecheck and both focused browser tests passed. A live loopback smoke test verified two-title evidence output. Hosted checks pending.

Validation for `f55c4e9` (PR #53): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45414917744) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45414917674) passed. The PR merged as `c976376`.

## 2026-11-24 — Model-assisted catalog field selection

- Added an optional Responses API selector behind a verified-session endpoint, owner and process limits, and a dashboard opt-in. The model selects allowed fields while the application supplies facts and provenance.
- Local Go checks, PostgreSQL authorization and admission limits under the race detector, frontend unit tests, build, typecheck and both analyst browser tests passed. Provider transport was synthetic. Hosted checks pending.

Validation for `eac3f43` (PR #54): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45415671881) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45415671820) passed. The PR merged as `b6859fb`.

## 2026-11-24 — Two-title catalog comparisons

- Added a second-title catalog picker, side-by-side sourced facts and numeric comparisons only where both episode counts or release years are known.
- Local Go checks, frontend unit tests, build, typecheck and both focused browser tests passed. Hosted checks pending.

Validation for `c405f68` (PR #55): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45416542335) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45416542427) passed. The PR merged as `87079b2`.

## 2026-11-24 — Loopback ingress rehearsal

- Added a disabled loopback Nginx example and an isolated container smoke check. The example bounds credential and assisted-analysis request intake independently and strips inbound forwarding headers.
- The local Docker smoke test passed with the pinned Nginx 1.30 image. The test exercised allowed traffic, shared identity-path throttling, local dashboard CORS and unaffected catalog and analysis requests. No listener was published. Hosted checks pending.

Validation for `063a4c5` (PR #56): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45417387538) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45417387536) passed, including the ingress rehearsal. The PR merged as `61f0612`.

## 2026-11-24 — Shared analyst rate budgets

- Moved the per-owner and deployment request budgets into an atomic PostgreSQL reservation. The per-process concurrency bound still applies. Owner budget records are forced-RLS scoped and included in account export and deletion.
- Local `make check` passed. The real PostgreSQL regression passed three runs under the race detector with separate router instances, owner and deployment caps, and private usage export and deletion. Hosted checks pending.

Validation for `ac8610f` (PR #57): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45418221160) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45418221069) passed, including explicit migrations, database integration and logical recovery. The full local PostgreSQL runtime suite also passed under the race detector. The PR merged as `5b6d2ca`.

## 2026-11-24 — Discovery beyond library evidence

- Added an owner-scoped mode that ranks by eligible positive features missing from the titles in the current library. Cursor mode and feature filter stay bound to each page.
- Local Go checks, both focused PostgreSQL recommendation tests under the race detector, frontend unit tests, build, typecheck and all five focused browser tests passed. Hosted checks pending.

Validation for `caef3cf` (PR #58): [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45419074034) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45419073827) passed. The PR merged as `700bf23`.

## 2026-11-24 — Overlapping franchise performance

- Coalesced equivalent calendar paths and aggregated eligible release targets by title, track and scope before joining events. The earliest valid follow and consent boundaries retain the existing eligibility rules.
- Local Go checks and linked-season, schedule consent and 128-title/80-follow PostgreSQL regressions passed. Across three local race runs per revision, median synthetic calendar reads fell from 72.4 to 7.3 ms and three-poll alert catch-up fell from 573.7 to 236.0 ms. These measurements are from the disposable fixture. Hosted checks pending.

Validation for PR #59 passed in [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45419831716) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45419831838). It merged as `a459fda`.

## 2026-11-24 — Shared SMTP transport

- Extracted the authenticated, certificate-verified SMTP transport for reuse by future release delivery. Identity message rendering and proof handling stay in the identity mail package.
- Local `make check` passed. Hosted checks pending.

## 2026-11-25 — Release email consent

- Added a verified-owner opt-in API, account export and restored-consent cleanup. Migration 000026 defaults everyone to opted out.
- Local make check and the focused PostgreSQL API race test passed. Hosted checks pending.

Consent PR #61 merged as `0aadc07` after [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45615029251) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45615029322) passed. The full local PostgreSQL runtime suite passed under the race detector.

## 2026-11-25 — Background release email

- Added the default-off SMTP worker, owner-scoped durable leases, bounded retries, deduplication, dashboard consent controls and recovery cleanup. Test mail stayed in a synthetic loopback mailbox.
- Local Go checks, PostgreSQL lease and release tests, frontend tests, build, typecheck and nine focused browser tests passed. Hosted checks pending.

Background delivery PR #62 merged as `bf99c3b` after [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45615977242) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45615977406) passed. An additional local smoke drove the running API through consent, automatic delivery into the loopback TLS mailbox, durable acknowledgement and opt-out.

## 2026-11-25 — Release email scheduler diagnostics

- Added read-only JSON and Prometheus exposition for aggregate scheduler state.
- Local make check, focused PostgreSQL race tests and CLI output smoke passed. Hosted checks pending.

Release diagnostics PR #63 merged as `60c6745` after [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45616503283) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45616503218) passed.

## 2026-11-25 — Bulk progress

- Added an atomic one-episode advance for selected count-mode tracker entries. Viewing passes and expected counts fence retries, while exact progress and known totals are protected.
- Local make check, focused PostgreSQL race tests, frontend unit tests, build, typecheck and the focused bulk-edit browser test passed. Hosted checks pending.

Bulk progress PR #64 merged after [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45617074058) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45617074127) passed.

## 2026-11-25 — Reviewed title cancellations

- Added explicit offline publication for authoritative primary title cancellations and scoped in-app notices for opted-in followers.
- Local Go checks, three focused PostgreSQL race tests, typecheck and all nine release browser tests passed. Hosted checks pending.

## 2026-11-25 — Application container smoke checks

- Added a hosted smoke check for both application images, non-root startup and loopback readiness. Compose gates web startup on API readiness and binds published ports to loopback.
- Compose validation and local image startup passed. The local dependency build used the Cloud proxy certificate through temporary BuildKit secrets. Hosted verification uses the committed Dockerfiles directly.

## 2026-11-25 — Concurrent dashboard workload

- Added a restricted PostgreSQL integration workload with eight synthetic owners, 1,000 catalog titles and 128 loopback HTTP requests across public searches, private library reads and competing progress updates. Each paired update commits once and conflicts once, and every library response contains only its owner's entry.
- Three local race-detector runs passed. Individual request workloads finished in 0.44–0.56 seconds in the disposable environment. The test uses a bounded context to detect hangs.
