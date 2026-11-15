# PostgreSQL development runtime

This is a local development setup, not permission to deploy publicly. Use synthetic accounts only until the remaining launch requirements in [IDENTITY_SECURITY.md](IDENTITY_SECURITY.md) are complete.

## Database setup

1. Use PostgreSQL 17 and Go 1.26+. Create an empty development database.
2. Supply its administrator/owner connection through `MIGRATION_DATABASE_URL` in your shell or a secret manager, then run `go run ./cmd/migrate`. Do not put credentials in files tracked by Git. The runner embeds all up migrations, records SHA-256 checksums in `schema_migrations`, and applies the schema and ledger atomically under an advisory lock. Re-running it is a no-op. Unknown applied versions, gaps, or edited migration checksums fail closed. Existing databases created with ad hoc SQL need a separately reviewed adoption process; do not fabricate ledger rows.
3. Run `deployments/runtime-role.sql` as the administrator, in that database. It creates `anime_runtime` with only the permissions needed by the implemented API. The script is safe to re-run to apply newly required grants to an existing runtime role. Set its password out of band, for example with interactive psql `\password anime_runtime`. Do not make this role a member of an owner or privileged role.
4. Supply the restricted role's connection as `DATABASE_URL` and run `go run ./cmd/api`. The default listener is `127.0.0.1:8080`; `HTTP_ADDR` overrides it. Never pass migration-owner credentials to the API. Startup rejects superuser, BYPASSRLS, role/database creation or replication privileges, inherited/public-table ownership, missing forced RLS, and the missing login-budget migration. Connection failure stops the process without printing the connection URL.

The API does not run migrations automatically. Upgrading the schema is an explicit administrator operation. The migration runner currently supports forward application; rollback scripts are exercised separately in CI. Production rollback and backup policy remain pending.

Without a non-empty `DATABASE_URL`, the API uses the original single-user JSON mode. Local Docker Compose ports bind only to the host loopback interface. The dashboard detects the runtime mode from `GET /api/v1`, and shows a login/registration form in PostgreSQL mode. Credentials live only in memory; reloading requires signing in again. A reload does not revoke the server-side session, which expires after 24 hours.

## Verified-email access

All PostgreSQL tracker reads, creates, updates and ratings require email verification. New and existing unverified accounts receive 403 `email_verification_required`; no automatic backfill or opt-out is provided. Registration/login still issue a session so the user can request verification and use account controls. Disabling SMTP leaves unverified library access locked. Configure delivery below to complete verification. Session controls, password recovery/change, export and deletion remain accessible before verification; catalog reads remain public. The JSON development runtime is unchanged.

## Authentication API

All paths start with `/api/v1`. Registration and login accept exactly one JSON object with `email` and `password`. Emails are normalized to lowercase; registration passwords must be 12–1024 bytes. No password/token/body is logged.

| Method | Path | Behavior |
| --- | --- | --- |
| POST | `/auth/register` | Creates an account and returns a new session; 201 |
| POST | `/auth/login` | Verifies credentials and returns a new session; 200 |
| GET | `/auth/me` | Returns the authenticated account, including emailVerified; 200 |
| POST | `/auth/refresh` | Atomically revokes the presented token and issues its replacement; 200 |
| POST | `/auth/logout` | Revokes the presented token; 204 |
| POST | `/auth/password` | Verifies `currentPassword`, hashes `newPassword`, and invalidates every existing account session; 204 |
| GET | `/auth/sessions` | Lists up to 100 owned active sessions, current first, with `hasMore`; no token/digest fields; 200 |
| DELETE | `/auth/sessions/{id}` | Revokes an owned active session; foreign/unknown/inactive IDs all return 404; 204 on success |
| POST | `/auth/sessions/revoke-others` | Revokes all other active sessions, including those beyond the listing limit; returns `revoked` count; 200 |
| GET | `/auth/export` | Downloads a consistent JSON snapshot of the current account's product data, excluding credential/session hashes; 200 |
| DELETE | `/auth/account` | Requires a JSON password confirmation; permanently deletes the current account, sessions, and owned records; 204 |

A session response contains `token`, `expiresAt`, and `user` (`id`, `email`, `displayName`, `emailVerified`). Tokens carry 256 random bits and expire in 24 hours. They are returned only at issuance; only SHA-256 digests are stored. Send the token using `Authorization: Bearer <token>`. Keep it in volatile client memory; never persist it in localStorage, URLs, source files, or logs. Cookies and URL/query credentials are not accepted.

`/auth/me`, `/auth/refresh`, `/auth/logout`, `/auth/export`, `/auth/account`, `/tracker`, `/tracker/{id}`, and `/ratings/{id}` require an authenticated bearer session. Catalog endpoints remain shared and readable without authentication. Expired, revoked, malformed, disabled-account, or locked-account sessions are rejected. Refresh is single use, including under concurrent requests; other independently issued sessions remain valid. Rotation preserves the session ID and original creation time, replacing the old digest and extending expiry to 24 hours from rotation. Revoking a listed ID therefore also invalidates any replacement credential issued by a racing refresh. Session listing and revocation use the verified caller as owner; request-supplied user IDs are never used. Logout revokes only the current session.

Private queries have explicit owner predicates and forced RLS. Session resolution first reads a server-computed digest using the narrow bootstrap policy, then sets transaction-local identity and locks/rechecks the session before private access. Commit or rollback clears the identity. No request-provided user ID is trusted. Cross-user entry updates return 404.

Registration/login are bounded by a process-wide token bucket and a two-operation hashing concurrency limit. Generic login errors and dummy verification reduce account enumeration. These controls are a development baseline; distributed/source abuse protection, queue monitoring, and independent review are required before public launch.

## Verification

Run `go test -race ./...`, `go vet ./...`, and `go build ./...`. In `web`, run `npm ci`, `npm test`, `npm run typecheck`, `npm run build`, then `npx playwright install chromium` and `npm run test:e2e`. Browser tests mock the API to test UI isolation and local-mode compatibility; database integration tests separately exercise the real API and PostgreSQL. A separate full-stack suite runs the built dashboard against the real Go API and PostgreSQL. The standard browser suite uses mocks for focused UI/session behavior.

To include real database tests, set `TEST_DATABASE_URL` to a test administrator connection and run `go test -race -count=1 ./internal/platform/postgres`. The administrator must be allowed to create/drop disposable databases and roles. Tests create randomly named isolated databases and non-owner roles, use synthetic `.test` accounts, and clean them up. They never drop the database named in the supplied connection.

GitHub Actions runs this suite against PostgreSQL 17, alongside the existing migration/rollback checks. It verifies auth success/failure, disabled/expired/revoked sessions, concurrent rotation, persistence, repeated rating upserts, HTTP auth boundaries, direct RLS rejection of cross-user reads/writes, transaction cleanup after errors, runtime role restrictions, and migration integrity.

Account export/deletion are available through the signed-in dashboard and API. Deletion requires the current password in the request body and cannot target a supplied user ID. It permanently removes all the account's product rows and sessions; other accounts and the shared catalog remain untouched. Reauthentication failures leave the account unchanged. The exported snapshot includes tracker, ratings, progress, preferences, recommendations, feedback, follows, and notifications, not password hashes or session digests.

In PostgreSQL mode, select **Manage sessions** while signed in to see the current and other active logins. **Revoke** signs out one other session. **Sign out other sessions** covers all other active logins, even those outside the 100-row listing. No tokens or digests are displayed. The current session stays signed in until the normal sign-out control is used or its server credential expires/is revoked.

## Full-stack browser verification

Use a disposable PostgreSQL database with the schema and restricted runtime role provisioned as above. Export its restricted `DATABASE_URL`; keep administrator credentials separate. Both full-stack suites now use the synthetic TLS mailbox to prove ownership of their fixture addresses. Configure `IDENTITY_MAIL_TEST=1`, `MAIL_SMTP_ADDR=127.0.0.1:8465`, `MAIL_SMTP_USER=synthetic-mailer`, `MAIL_FROM=sender@example.test`, `PUBLIC_APP_ORIGIN=http://127.0.0.1:3000`, fresh random `MAIL_SMTP_PASSWORD`/`MAIL_TEST_ACCESS_KEY`, and `MAIL_SMTP_CA_FILE` inside a private disposable temporary directory. Playwright starts `cmd/testmail`, which creates the public test CA before the API starts. See the CI setup step for the complete automatic provisioning example. Never use real accounts or real provider credentials with this fixture.

Make ports 3000, 8080, 8026 and 8465 available, install the Chromium test browser, and run in `web`:

```sh
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080/api/v1 npm run build
E2E_REAL_API=1 CI=1 NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080/api/v1 npm run test:e2e
```

Playwright starts both the Go API and production Next.js server on loopback. The suite uses random synthetic `.test` accounts and exercises real registration/login, tracker progress and ratings, individual/bulk remote revocation, browser cleanup after remote revocation, cross-account history/update isolation, export, and password-confirmed deletion. It does not mock network responses. The default suite (`npm run test:e2e` without `E2E_REAL_API`) keeps the focused mocked tests.

The dedicated CI job provisions an ephemeral PostgreSQL 17 database, applies the embedded migrations and actual `deployments/runtime-role.sql`, generates a temporary masked runtime password, and runs the browser with that non-owner connection. No public deployment or external provider access is involved. Test reports/screenshots contain synthetic fixtures only and are retained for three days.

The signed-in dashboard's **Change password** form requires the current password and a 12–1024-byte replacement. Success revokes every existing session and returns the dashboard to sign-in. Incorrect current passwords leave the session intact, show an inline error, and clear both password inputs. Forgotten-password recovery and email verification use the explicitly configured identity mail flow below. Password changes share the bounded authentication hash/rate gate and reject request-provided owner fields.

Migration 000006 adds purpose-bound identity challenges with forced RLS. Re-run the runtime-role grant script after applying it. Current API startup requires forced RLS on all thirteen private tables, including the migration 000009 request inbox. Existing credential changes invalidate pending identity challenges. Raw identity tokens are delivered asynchronously through an explicitly configured SMTP adapter; they are never returned through application JSON or logs.

## Identity mail configuration

Delivery is disabled by default. To enable it, set all of `MAIL_SMTP_ADDR` (`host:port`, implicit TLS), `MAIL_SMTP_USER`, `MAIL_SMTP_PASSWORD`, `MAIL_FROM` (bare sender address), `MAIL_OUTBOX_KEY` (base64-encoded 32-byte deployment key), and `PUBLIC_APP_ORIGIN` (trusted HTTPS origin; HTTP is accepted only for loopback development). Optional `MAIL_SMTP_CA_FILE` adds a trusted PEM CA without disabling certificate or hostname checks. Partial or invalid configuration stops startup without logging the values. The API never derives links from request Host headers.

`POST /auth/password-reset/request` accepts `{email}` and returns the same 202 acceptance for known/unknown/disabled accounts. A fixed-size encrypted request intent commits before the response; account lookup, issuance and SMTP run separately. The inbox holds at most 256 active requests across instances, independent of account existence. If delivery is disabled/unavailable, all recipients get the same 503. `POST /auth/email-verification/request` requires an authenticated session and sends only to its account's stored email; client recipient/owner fields are rejected. Both flows have persisted per-account/purpose resend limits: one minute between issues and at most five per hour.

`POST /auth/password-reset/confirm` accepts `{token,newPassword}`; `POST /auth/email-verification/confirm` accepts `{token}`. Confirmation is an explicit POST, works without an active session, and never automatically logs in. Tokens expire after 30 minutes for reset or one hour for verification. They are random, purpose-bound, single-use, tied to the stored address, and invalidated by password changes/reset/deletion. Reset revokes all old sessions and clears temporary login locks. Verification updates `emailVerified`; all PostgreSQL tracker and rating endpoints require that flag.

Mail links put credentials in fragments. The client must scrub fragments, keep proof only in memory, and require explicit confirmation. The request inbox and delivery outbox survive restart. Inbox consumption, proof issuance and encrypted delivery insertion commit atomically; SMTP failures are logged only as generic events. Configured SMTP must be validated operationally before launch; no real email is sent by development tests.

When mail is configured, **Forgot password?** requests a generic recovery email, and signed-in unverified accounts can request verification. Links load a confirmation form and are removed from browser history before proof submission. Nothing happens merely by loading a link. After reset, sign in again with the replacement password. Verification is recorded in `emailVerified`; the existing bearer can then access tracker and rating endpoints without signing in again.

CI runs two isolated full-stack variants. `E2E_SUITE=postgres` covers the tracker/account baseline. `E2E_SUITE=identity` starts `cmd/testmail` before the API, writes its generated public CA before reporting readiness, and tests the actual SMTP/queue/database/browser proof flow. The fixture requires `IDENTITY_MAIL_TEST=1`, explicit SMTP credentials, `MAIL_TEST_ACCESS_KEY`, and `MAIL_SMTP_CA_FILE`; it listens only on numeric loopback addresses and never contacts external providers. Mailbox reads require the test access key. All certificates/passwords/mailboxes are disposable, and the API image includes none of the fixture server.


Signed-in dashboard users can download a JSON account export from **Account data**. The downloaded file contains private product data and should be kept private. The dashboard does not persist the export in browser storage. **Delete account** requires the current password and an explicit permanent-deletion acknowledgement. Failed password confirmation keeps the valid session; successful deletion clears the dashboard and invalidates every session and identity challenge through database cascades.


## Persisted login-attempt budget

Apply migration `000007_login_attempt_budget` before updating the API; startup checks that the security column exists. A credentialed account's failed login or incorrect current-password confirmation for password change/deletion increments the same database counter under an exclusive account lock. Ten failures within a fifteen-minute observation window lock authentication for fifteen minutes. Multiple API instances share this state, and process restarts do not reset it. Expired observation windows start fresh. Requests during an active lock do not extend it; the existing private-session checks also reject a locked account.

Successful login, password change and password reset clear the counter, observation start and lock. Recovery requests and password-reset proofs remain usable while locked, allowing address owners to recover. Unknown, disabled, locked and incorrect-password login responses remain the same generic 401. Incorrect sensitive-action confirmations return `invalid_current_password` to an already authenticated caller; active account locks return the normal session rejection. Reauthentication failures commit the shared budget before returning and leave valid sessions intact until the threshold is reached. No IP address or user-agent data is collected. The process-wide request/hash gate still applies.

This baseline account lock can itself be abused to deny access to a known account. Public launch still needs reviewed ingress controls, distributed source throttling, monitoring and an operational recovery path. It is not a replacement for those controls.


### Encrypted durable delivery (migration 000008)

Apply migration `000008_identity_outbox`, then reapply `deployments/runtime-role.sql` before updating the API. Current startup requires migration 000010 and forced RLS on all thirteen private tables. When enabling SMTP, also set `MAIL_OUTBOX_KEY` to a securely generated base64-encoded 32-byte key (for example generated out of band with `openssl rand -base64 32`). Missing/invalid keys stop startup. Never commit the key or store it in the database; all replicas must use the same key. Back it up separately from database backups. Optional `MAIL_OUTBOX_PREVIOUS_KEYS` provides up to three distinct comma-separated decryption keys; new encryption always uses `MAIL_OUTBOX_KEY`. Read [key rotation](IDENTITY_KEY_ROTATION.md) before changing keys: prepare every reader before switching writers, then retire old keys only after pending payloads have cleared. Secret distribution/rollout is not automatic. A valid-looking wrong key preserves pending ciphertext and stops delivery until corrected or expired.

Challenge issuance and AES-256-GCM encrypted delivery insertion share one transaction. The envelope is bound to the challenge ID/version and checked against its purpose, digest, address and expiry before claim. Workers acquire account/challenge/outbox locks in order, persist a one-minute hashed lease, then release all database locks before SMTP. Five persisted claims are allowed, including crashed attempts; failures use 30/60/120/240-second backoffs. Each send is capped at fifteen seconds and proof expiry. A restarted worker resumes pending work without creating another proof or spending resend quota. Stale acknowledgements cannot erase a newer claim.

Successful DATA acceptance ends delivery even if QUIT fails. Shutdown still attempts a bounded acknowledgement after accepted delivery. Ambiguous failures or a crash between SMTP acceptance and acknowledgement can duplicate mail: this is at-least-once delivery, never exactly-once. Completed/exhausted/expired/invalid jobs erase ciphertext; completed metadata is purged after one day in bounded worker batches. Account deletion cascades the outbox. Outbox data is excluded from HTTP responses and account exports.

The independent `cmd/worker` remains a placeholder; configured API instances poll both durable queues. Queue monitoring, automated secret rollout, recovery exercises and provider operations remain launch work.

### Durable request acceptance (migration 000009)

Apply `000009_identity_requests` and reapply the runtime-role grants before updating the API, even when mail is disabled. Configured mail uses the durable service exclusively. HTTP 202 means an encrypted request intent committed, not that an account exists or mail was sent. Database failure, disabled delivery, or full shared capacity returns a generic 503 for both known and unknown accounts. Admission has a five-second timeout, respects request cancellation, and is cancelled/joined during service shutdown. A lost response or ambiguous commit can still cause a client retry to submit another intent; issuance quotas remain shared.

The inbox allows 256 active requests across API pools/replicas using a serialized admission lock. Admission never queries accounts; every intent has a fixed 540-byte encrypted payload, preventing length-based classification of known/unknown recipients. Requests expire after fifteen minutes. Reset intents contain the submitted email. Verification intents contain only the presented session's SHA-256 digest, encrypted with the same deployment key under a distinct authenticated domain; raw bearer tokens are never persisted. The worker rechecks current session/account state, so logout, refresh, password changes, expiry, disablement or deletion can invalidate queued verification.

Workers claim a one-minute hashed lease, release that transaction, then acquire account/session state before the request row and proof/outbox records. Inbox completion, proof issuance and delivery insertion commit together. Failure rolls all three back and schedules persisted 5/10/20/40-second retries; five claims include crashes. Wrong keys preserve pending ciphertext and fail closed. Completion, expiry or exhaustion erases payloads, and workers purge completed metadata after one day in bounded batches. These are processing deadlines and retry limits, not a guarantee of eventual delivery.

Request rows are anonymous operational intents, excluded from account export and HTTP responses. Their encrypted payload has no account foreign key because admission cannot look up account existence; deletion may leave a pending anonymous reset intent until processing/expiry. Processing always uses live account state. Protect keys separately from database backups, monitor both queues, and follow the staged keyring rollout/retirement procedure for rotation. The volatile `NewService` adapter remains for local tests only; `cmd/api` does not use it.

### Bounded decryption keyring

`MAIL_OUTBOX_PREVIOUS_KEYS` is optional, with at most three canonical base64-encoded 32-byte keys separated by commas. The primary/additional keys must be distinct; empty entries, whitespace and malformed values stop startup. Primary-only configuration remains supported, and existing ciphertext requires no migration. Both inbox and outbox encryption always uses the primary; every decryption still checks authenticated purpose/domain and row binding. The [rotation runbook](IDENTITY_KEY_ROTATION.md) describes mixed-node compatibility, cutoff-based retirement and rollback without exposing private queue payloads.

### Operator queue snapshot

`go run ./cmd/identity-status` uses the restricted `DATABASE_URL` to report versioned aggregate JSON from one read-only repeatable-read snapshot. It requires neither mail credentials nor queue keys, never decrypts private envelopes or starts workers, and adds no HTTP route. Read [IDENTITY_QUEUE_OPERATIONS.md](IDENTITY_QUEUE_OPERATIONS.md) for exact fields and limitations: a successful read is not a provider-health verdict, completed work is not synonymous with successful delivery, and monitoring/alerts remain pending.

### Queue outcomes (migration 000010)

Apply `000010_identity_queue_outcomes` before updating the API or diagnostic command. No new table grants are needed if migration 000009 grants are already current. Startup checks both outcome columns and still requires thirteen forced-RLS tables. New workers atomically record allowlisted terminal outcomes with completion/payload erasure. Unknown/no-action request reasons stay merged, and no provider responses or private values are stored as outcome text. Historical and rolling old-writer completions are explicitly unknown. See [queue operations](IDENTITY_QUEUE_OPERATIONS.md) for exact fields, lease fencing, retention and gauge semantics.

`go run ./cmd/identity-status -format=prometheus` renders the same snapshot as a fixed, bounded set of gauge series. It adds no HTTP route or automatic collector/alert setup. Use atomic textfile replacement and monitor the snapshot timestamp; retained outcomes must not be interpreted as cumulative counters. See [queue operations](IDENTITY_QUEUE_OPERATIONS.md) for exact names, zero/age semantics, privacy limits and shared-database deduplication.

Atomic collector mode is available as `go run ./cmd/identity-status -format=prometheus -output-file=/absolute/path.prom`. It emits no snapshot to stdout, preserves a previous export on pre-rename failure, and never follows the old destination symlink. The [disabled monitoring examples](../deployments/monitoring/README.md) include a hardened oneshot/timer, loopback scrape template and promtool-tested alerts. Operator review/authorization is required before host activation; nothing is installed by this command.

## Liveness, readiness and request logs

For isolated logical restore, restricted-role re-provisioning, queue-key recovery and offline credential fencing, see [backup recovery](BACKUP_RECOVERY.md). The dedicated CI recovery job uses synthetic databases only; real backup storage, post-snapshot deletion/security-event reconciliation and PITR remain operational work.

`GET /healthz` confirms the process is responding and stays independent of PostgreSQL availability. `GET /readyz` returns 200 only when a live pool connection still uses a restricted non-owner role and the minimum required security schema is present: thirteen forced-RLS tables, login-budget and identity-queue outcome columns. It shares the startup validation path. Failure returns generic 503 with no connection, schema or role details. Responses carry `Cache-Control: no-store`; the request deadline is capped at two seconds and overlapping database probes are rejected without queuing. Caller cancellation is respected. Local JSON mode reports readiness without an external dependency.

Use readiness for dependency-aware routing and liveness for process checks during an authorized operational rollout. Keep probes on trusted ingress and account for the one-probe concurrency limit. A probe is a momentary check, not a replacement for transaction authorization, correct RLS policies, continuous monitoring, mail-provider checks or independent security review. No deployment/proxy configuration is enabled by these endpoints.

Request log field `path` is replaced by `route`, the matched ServeMux template (for example `GET /api/v1/anime/{id}`) or `unmatched`. Standard method values are retained; custom methods become `OTHER`. Duration remains numeric. URL paths/parameters/query values, request bodies, headers, cookies, credentials and user IDs are not logged by this middleware. Update any operational log queries that relied on raw paths.

## Response privacy headers

Both API routers apply `Cache-Control: no-store`, `X-Content-Type-Options: nosniff` and `Referrer-Policy: no-referrer` before route/CORS dispatch. This includes local mode, errors, redirects, HEAD and preflight responses as well as successful private responses. Existing per-handler no-store protections remain. Public catalog responses also opt out of caching; explicit public caching can be designed separately. These headers do not clear previously cached responses or control the dashboard's own document policy. No deployment or ingress policy is activated.

## Removing a library title

`DELETE /api/v1/tracker/{id}` returns an empty 204 for an owned entry; malformed IDs return 400, foreign/missing/already removed entries return the same 404. PostgreSQL requires a live session and verified email. The transaction removes the entry and all of that account's ratings/progress for its title; other accounts, catalog records, follows and recommendation history are preserved. Re-adding starts with fresh progress and no rating. Failure rolls back every deletion. JSON mode supports the same operation for its local single-user library, restoring memory if file persistence fails.

Existing installations must reapply `deployments/runtime-role.sql` to grant DELETE on `user_anime`, `watch_progress` and `user_ratings` to the restricted role. No new schema migration is required. Without those grants removal fails closed; do not elevate the runtime role. The dashboard provides an explicit removal confirmation describing ratings/progress deletion, with cancel/retry controls. Failed requests retain the entry; successful removal refreshes library totals and allows re-adding. Confirmation is cleared when accounts change, and late mutation/refresh completions are fenced.
