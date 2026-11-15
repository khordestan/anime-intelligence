# PostgreSQL development runtime

This is a local development setup, not permission to deploy publicly. Use synthetic accounts only until the remaining launch requirements in [IDENTITY_SECURITY.md](IDENTITY_SECURITY.md) are complete.

## Database setup

1. Use PostgreSQL 17 and Go 1.26+. Create an empty development database.
2. Supply its administrator/owner connection through `MIGRATION_DATABASE_URL` in your shell or a secret manager, then run `go run ./cmd/migrate`. Do not put credentials in files tracked by Git. The runner embeds all up migrations, records SHA-256 checksums in `schema_migrations`, and applies the schema and ledger atomically under an advisory lock. Re-running it is a no-op. Unknown applied versions, gaps, or edited migration checksums fail closed. Existing databases created with ad hoc SQL need a separately reviewed adoption process; do not fabricate ledger rows.
3. Run `deployments/runtime-role.sql` as the administrator, in that database. It creates `anime_runtime` with only the permissions needed by the implemented API. The script is safe to re-run to apply newly required grants to an existing runtime role. Set its password out of band, for example with interactive psql `\password anime_runtime`. Do not make this role a member of an owner or privileged role.
4. Supply the restricted role's connection as `DATABASE_URL` and run `go run ./cmd/api`. The default listener is `127.0.0.1:8080`; `HTTP_ADDR` overrides it. Never pass migration-owner credentials to the API. Startup rejects superuser, BYPASSRLS, role/database creation or replication privileges, inherited/public-table ownership, and missing forced RLS. Connection failure stops the process without printing the connection URL.

The API does not run migrations automatically. Upgrading the schema is an explicit administrator operation. The migration runner currently supports forward application; rollback scripts are exercised separately in CI. Production rollback and backup policy remain pending.

Without a non-empty `DATABASE_URL`, the API uses the original single-user JSON mode. Local Docker Compose ports bind only to the host loopback interface. The dashboard detects the runtime mode from `GET /api/v1`, and shows a login/registration form in PostgreSQL mode. Credentials live only in memory; reloading requires signing in again. A reload does not revoke the server-side session, which expires after 24 hours.

## Authentication API

All paths start with `/api/v1`. Registration and login accept exactly one JSON object with `email` and `password`. Emails are normalized to lowercase; registration passwords must be 12–1024 bytes. No password/token/body is logged.

| Method | Path | Behavior |
| --- | --- | --- |
| POST | `/auth/register` | Creates an account and returns a new session; 201 |
| POST | `/auth/login` | Verifies credentials and returns a new session; 200 |
| GET | `/auth/me` | Returns the verified account; 200 |
| POST | `/auth/refresh` | Atomically revokes the presented token and issues its replacement; 200 |
| POST | `/auth/logout` | Revokes the presented token; 204 |
| POST | `/auth/password` | Verifies `currentPassword`, hashes `newPassword`, and invalidates every existing account session; 204 |
| GET | `/auth/sessions` | Lists up to 100 owned active sessions, current first, with `hasMore`; no token/digest fields; 200 |
| DELETE | `/auth/sessions/{id}` | Revokes an owned active session; foreign/unknown/inactive IDs all return 404; 204 on success |
| POST | `/auth/sessions/revoke-others` | Revokes all other active sessions, including those beyond the listing limit; returns `revoked` count; 200 |
| GET | `/auth/export` | Downloads a consistent JSON snapshot of the current account's product data, excluding credential/session hashes; 200 |
| DELETE | `/auth/account` | Requires a JSON password confirmation; permanently deletes the current account, sessions, and owned records; 204 |

A session response contains `token`, `expiresAt`, and `user` (`id`, `email`, `displayName`). Tokens carry 256 random bits and expire in 24 hours. They are returned only at issuance; only SHA-256 digests are stored. Send the token using `Authorization: Bearer <token>`. Keep it in volatile client memory; never persist it in localStorage, URLs, source files, or logs. Cookies and URL/query credentials are not accepted.

`/auth/me`, `/auth/refresh`, `/auth/logout`, `/auth/export`, `/auth/account`, `/tracker`, `/tracker/{id}`, and `/ratings/{id}` require a verified bearer session. Catalog endpoints remain shared and readable without authentication. Expired, revoked, malformed, disabled-account, or locked-account sessions are rejected. Refresh is single use, including under concurrent requests; other independently issued sessions remain valid. Rotation preserves the session ID and original creation time, replacing the old digest and extending expiry to 24 hours from rotation. Revoking a listed ID therefore also invalidates any replacement credential issued by a racing refresh. Session listing and revocation use the verified caller as owner; request-supplied user IDs are never used. Logout revokes only the current session.

Private queries have explicit owner predicates and forced RLS. Session resolution first reads a server-computed digest using the narrow bootstrap policy, then sets transaction-local identity and locks/rechecks the session before private access. Commit or rollback clears the identity. No request-provided user ID is trusted. Cross-user entry updates return 404.

Registration/login are bounded by a process-wide token bucket and a two-operation hashing concurrency limit. Generic login errors and dummy verification reduce account enumeration. These controls are a development baseline; distributed/per-account abuse protection, recovery/verification, and independent review are required before public launch.

## Verification

Run `go test -race ./...`, `go vet ./...`, and `go build ./...`. In `web`, run `npm ci`, `npm test`, `npm run typecheck`, `npm run build`, then `npx playwright install chromium` and `npm run test:e2e`. Browser tests mock the API to test UI isolation and local-mode compatibility; database integration tests separately exercise the real API and PostgreSQL. A separate full-stack suite runs the built dashboard against the real Go API and PostgreSQL. The standard browser suite uses mocks for focused UI/session behavior.

To include real database tests, set `TEST_DATABASE_URL` to a test administrator connection and run `go test -race -count=1 ./internal/platform/postgres`. The administrator must be allowed to create/drop disposable databases and roles. Tests create randomly named isolated databases and non-owner roles, use synthetic `.test` accounts, and clean them up. They never drop the database named in the supplied connection.

GitHub Actions runs this suite against PostgreSQL 17, alongside the existing migration/rollback checks. It verifies auth success/failure, disabled/expired/revoked sessions, concurrent rotation, persistence, repeated rating upserts, HTTP auth boundaries, direct RLS rejection of cross-user reads/writes, transaction cleanup after errors, runtime role restrictions, and migration integrity.

Account export/deletion currently have API endpoints only. Deletion requires the current password in the request body and cannot target a supplied user ID. It permanently removes all the account's product rows and sessions; other accounts and the shared catalog remain untouched. Reauthentication failures leave the account unchanged. The exported snapshot includes tracker, ratings, progress, preferences, recommendations, feedback, follows, and notifications, not password hashes or session digests.

In PostgreSQL mode, select **Manage sessions** while signed in to see the current and other active logins. **Revoke** signs out one other session. **Sign out other sessions** covers all other active logins, even those outside the 100-row listing. No tokens or digests are displayed. The current session stays signed in until the normal sign-out control is used or its server credential expires/is revoked.

## Full-stack browser verification

Use a disposable PostgreSQL database with the schema and restricted runtime role provisioned as above. Export its restricted `DATABASE_URL`; keep administrator credentials separate. Make ports 3000 and 8080 available, install the Chromium test browser, and run in `web`:

```sh
NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080/api/v1 npm run build
E2E_REAL_API=1 CI=1 NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:8080/api/v1 npm run test:e2e
```

Playwright starts both the Go API and production Next.js server on loopback. The suite uses random synthetic `.test` accounts and exercises real registration/login, tracker progress and ratings, individual/bulk remote revocation, browser cleanup after remote revocation, cross-account history/update isolation, export, and password-confirmed deletion. It does not mock network responses. The default suite (`npm run test:e2e` without `E2E_REAL_API`) keeps the focused mocked tests.

The dedicated CI job provisions an ephemeral PostgreSQL 17 database, applies the embedded migrations and actual `deployments/runtime-role.sql`, generates a temporary masked runtime password, and runs the browser with that non-owner connection. No public deployment or external provider access is involved. Test reports/screenshots contain synthetic fixtures only and are retained for three days.

The signed-in dashboard's **Change password** form requires the current password and a 12–1024-byte replacement. Success revokes every existing session and returns the dashboard to sign-in. Incorrect current passwords leave the session intact, show an inline error, and clear both password inputs. Forgotten-password recovery and email verification use the explicitly configured identity mail flow below. Password changes share the bounded authentication hash/rate gate and reject request-provided owner fields.

Migration 000006 adds purpose-bound identity challenges with forced RLS. Re-run the runtime-role grant script after applying it. API startup now requires forced RLS on all eleven private tables. Existing credential changes invalidate pending identity challenges. Raw identity tokens are delivered asynchronously through an explicitly configured SMTP adapter; they are never returned through application JSON or logs.

## Identity mail configuration

Delivery is disabled by default. To enable it, set all of `MAIL_SMTP_ADDR` (`host:port`, implicit TLS), `MAIL_SMTP_USER`, `MAIL_SMTP_PASSWORD`, `MAIL_FROM` (bare sender address), and `PUBLIC_APP_ORIGIN` (trusted HTTPS origin; HTTP is accepted only for loopback development). Optional `MAIL_SMTP_CA_FILE` adds a trusted PEM CA without disabling certificate or hostname checks. Partial or invalid configuration stops startup without logging the values. The API never derives links from request Host headers.

`POST /auth/password-reset/request` accepts `{email}` and returns the same 202 acceptance for known/unknown/disabled accounts. Lookup and SMTP run in a bounded 32-item background queue, separately from the response. If delivery is disabled/unavailable, all recipients get the same 503. `POST /auth/email-verification/request` requires a verified session and sends only to its account's stored email; client recipient/owner fields are rejected. Both flows have persisted per-account/purpose resend limits: one minute between issues and at most five per hour.

`POST /auth/password-reset/confirm` accepts `{token,newPassword}`; `POST /auth/email-verification/confirm` accepts `{token}`. Confirmation is an explicit POST, works without an active session, and never automatically logs in. Tokens expire after 30 minutes for reset or one hour for verification. They are random, purpose-bound, single-use, tied to the stored address, and invalidated by password changes/reset/deletion. Reset revokes all old sessions and clears temporary login locks. Verification updates `emailVerified`; private development endpoints do not yet require that flag.

Mail links put credentials in fragments. The client must scrub fragments, keep proof only in memory, and require explicit confirmation. The delivery queue is volatile: restart/shutdown may drop pending work, SMTP failures are logged only as generic events, and automatic durable retries remain production work. Configured SMTP must be validated operationally before launch; no real email is sent by development tests.

When mail is configured, **Forgot password?** requests a generic recovery email, and signed-in unverified accounts can request verification. Links load a confirmation form and are removed from browser history before proof submission. Nothing happens merely by loading a link. After reset, sign in again with the replacement password. Verification is recorded in `emailVerified`; private development routes are not yet gated on that flag.

CI runs two isolated full-stack variants. `E2E_SUITE=postgres` covers the tracker/account baseline. `E2E_SUITE=identity` starts `cmd/testmail` before the API, writes its generated public CA before reporting readiness, and tests the actual SMTP/queue/database/browser proof flow. The fixture requires `IDENTITY_MAIL_TEST=1`, explicit SMTP credentials, `MAIL_TEST_ACCESS_KEY`, and `MAIL_SMTP_CA_FILE`; it listens only on numeric loopback addresses and never contacts external providers. Mailbox reads require the test access key. All certificates/passwords/mailboxes are disposable, and the API image includes none of the fixture server.
