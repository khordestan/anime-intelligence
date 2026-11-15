# PostgreSQL development runtime

This is a local development setup, not permission to deploy publicly. Use synthetic accounts only until the remaining launch requirements in [IDENTITY_SECURITY.md](IDENTITY_SECURITY.md) are complete.

## Database setup

1. Use PostgreSQL 17 and Go 1.26+. Create an empty development database.
2. Supply its administrator/owner connection through `MIGRATION_DATABASE_URL` in your shell or a secret manager, then run `go run ./cmd/migrate`. Do not put credentials in files tracked by Git. The runner embeds all up migrations, records SHA-256 checksums in `schema_migrations`, and applies the schema and ledger atomically under an advisory lock. Re-running it is a no-op. Unknown applied versions, gaps, or edited migration checksums fail closed. Existing databases created with ad hoc SQL need a separately reviewed adoption process; do not fabricate ledger rows.
3. Run `deployments/runtime-role.sql` as the administrator, in that database. It creates `anime_runtime` with only the permissions needed by the implemented API. Set its password out of band, for example with interactive psql `\password anime_runtime`. Do not make this role a member of an owner or privileged role.
4. Supply the restricted role's connection as `DATABASE_URL` and run `go run ./cmd/api`. The default listener is `127.0.0.1:8080`; `HTTP_ADDR` overrides it. Never pass migration-owner credentials to the API. Startup rejects superuser, BYPASSRLS, inherited/public-table ownership, and missing forced RLS. Connection failure stops the process without printing the connection URL.

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

A session response contains `token`, `expiresAt`, and `user` (`id`, `email`, `displayName`). Tokens carry 256 random bits and expire in 24 hours. They are returned only at issuance; only SHA-256 digests are stored. Send the token using `Authorization: Bearer <token>`. Keep it in volatile client memory; never persist it in localStorage, URLs, source files, or logs. Cookies and URL/query credentials are not accepted.

`/auth/me`, `/auth/refresh`, `/auth/logout`, `/tracker`, `/tracker/{id}`, and `/ratings/{id}` require a verified bearer session. Catalog endpoints remain shared and readable without authentication. Expired, revoked, malformed, disabled-account, or locked-account sessions are rejected. Refresh is single use, including under concurrent requests; other independently issued sessions remain valid. Logout revokes only the current session.

Private queries have explicit owner predicates and forced RLS. Session resolution first reads a server-computed digest using the narrow bootstrap policy, then sets transaction-local identity and locks/rechecks the session before private access. Commit or rollback clears the identity. No request-provided user ID is trusted. Cross-user entry updates return 404.

Registration/login are bounded by a process-wide token bucket and a two-operation hashing concurrency limit. Generic login errors and dummy verification reduce account enumeration. These controls are a development baseline; distributed/per-account abuse protection, recovery/verification, and independent review are required before public launch.

## Verification

Run `go test -race ./...`, `go vet ./...`, and `go build ./...`. In `web`, run `npm ci`, `npm test`, `npm run typecheck`, `npm run build`, then `npx playwright install chromium` and `npm run test:e2e`. Browser tests mock the API to test UI isolation and local-mode compatibility; database integration tests separately exercise the real API and PostgreSQL. Full-stack browser/database testing remains pending.

To include real database tests, set `TEST_DATABASE_URL` to a test administrator connection and run `go test -race -count=1 ./internal/platform/postgres`. The administrator must be allowed to create/drop disposable databases and roles. Tests create randomly named isolated databases and non-owner roles, use synthetic `.test` accounts, and clean them up. They never drop the database named in the supplied connection.

GitHub Actions runs this suite against PostgreSQL 17, alongside the existing migration/rollback checks. It verifies auth success/failure, disabled/expired/revoked sessions, concurrent rotation, persistence, repeated rating upserts, HTTP auth boundaries, direct RLS rejection of cross-user reads/writes, transaction cleanup after errors, runtime role restrictions, and migration integrity.
