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
