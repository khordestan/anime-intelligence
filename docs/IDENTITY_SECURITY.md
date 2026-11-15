# Identity and session security design

## Status

PostgreSQL mode implements registration, login, session resolution, logout, atomic token rotation, and user-scoped tracker persistence. CI executes these flows through a restricted PostgreSQL role, with direct RLS and HTTP isolation assertions. The JSON API remains unauthenticated and single-user. Neither mode is approved for public deployment; production abuse controls, account recovery/verification and independent review remain pending. See [runtime setup](POSTGRES_RUNTIME.md).

The current API uses explicit `Authorization: Bearer` headers, never cookies. The browser dashboard supports PostgreSQL login, registration, logout, and expiry, with tokens held only in component-scoped memory. Changing accounts aborts pending requests and rejects late results; reload clears the client credential and private library. The server session remains subject to its 24-hour expiry unless explicitly revoked. Raw tokens must remain in volatile client memory, never URLs, logs, or browser persistent storage.

## Credential storage

- Email addresses are required and unique case-insensitively.
- Passwords must be hashed in the application with Argon2id using parameters reviewed against current OWASP guidance and the deployment's resource budget.
- Store only the encoded Argon2id PHC string in `users.password_hash`; never store plaintext passwords or reversible encryption.
- Password verification must use a vetted library and constant-time comparison as provided by that library.
- Keep password reset and email verification tokens out of logs; persist only token digests with expiry and one-time-use semantics when those flows are added.

## Session contract

- Generate at least 256 bits of cryptographically secure random entropy for each opaque bearer token.
- Return the raw token once to the client; persist only its SHA-256 digest in `auth_sessions.token_hash`.
- Enforce expiry and revocation on every authenticated request. Update last-used metadata with a bounded cadence to avoid a write on every request.
- Revoke sessions on logout, password change/reset, account disablement, and relevant security events.
- Use secure, HttpOnly cookies with an appropriate SameSite policy in browser deployments. Add CSRF protection for cookie-authenticated state-changing requests. Never put bearer tokens in URLs or logs.
- Avoid persisting raw IP addresses or user-agent strings unless a documented security requirement and retention policy justify it.

## Authorization and tenant isolation

- Derive the user identity only from a verified server-side session. Never trust a client-supplied `user_id`, query parameter, or request-body owner field.
- During login, set `app.login_email` from the normalized submitted email only for the account lookup transaction. During session resolution, set `app.session_token_hash` to the server-computed SHA-256 hex digest of the presented token. After verification, set `app.user_id` to the session's user ID for protected queries. Clear context by ending the transaction.
- Every user-owned read, update, delete, and feedback query must include the authenticated `user_id` in its predicate.
- Migration `000005_row_level_security` enables and forces RLS on users, sessions, watch history, ratings, progress, preferences, recommendations, feedback, follows, and notifications. User-owned policies use the transaction-scoped `app.user_id` setting. Two narrowly scoped SELECT policies also support identity bootstrap: `app.login_email` can resolve one login account, and `app.session_token_hash` can resolve a session by the SHA-256 digest of the presented opaque token. These settings must be populated only by trusted server code from the login form or a server-computed token digest. The PostgreSQL runtime sets and clears these contexts inside transactions. Session bootstrap first resolves the digest, then establishes identity and rechecks the session under a row lock before private queries. Logout and refresh take the same lock; concurrent refresh can succeed only once. The application database role must not be a superuser or have `BYPASSRLS`; do not run the application as the schema owner.
- The `app.user_id` setting is a trusted server-side context, not an authentication mechanism. Only set it after verifying a server-side session, within the same transaction as the protected queries. Never derive it from request input.
- Test cross-user access attempts for every user-owned resource, not only happy paths.
- Keep catalog data public/shared; isolate watch history, ratings, notes, preferences, recommendation feedback, follows, and notifications.

## Abuse controls

- Add rate limits for login, registration, password reset, and token verification.
- Use generic login errors to avoid account enumeration.
- Apply progressive throttling/temporary lockout carefully; do not allow trivial denial-of-service against a known account.
- Emit security audit events without credentials or sensitive request bodies.
- Session listing, individual remote revocation, and revocation of other sessions are implemented through the PostgreSQL API. Account data export/deletion are implemented through the PostgreSQL API; dashboard controls remain pending.

## Public repository rules

Only schema, source code, synthetic fixtures, and documentation belong in this public repository. Real account details, personal anime histories, session tokens, password hashes from real accounts, production database dumps, and secrets must stay out of Git history.

## Implemented limits and remaining launch work

- Argon2id v19, 64 MiB, three iterations, parallelism two, 16-byte random salt, 32-byte output. Verification accepts only this bounded profile. Password input is 12–1024 bytes when registering.
- Authentication request bodies are limited to 4 KiB and reject unknown fields and trailing JSON. Login failures use generic responses, and missing accounts perform dummy hash verification.
- Registration/login share a process-wide burst of ten attempts, replenished at one attempt per six seconds, with at most two password operations in flight. This bounds memory use but does not replace distributed/per-account production controls; a global limiter can be exhausted by an attacker.
- Sessions expire after 24 hours. Refresh atomically revokes the old token and issues a new one. Disabled/locked accounts are checked on every private transaction. Last-used writes are limited to a five-minute cadence.
- API responses containing credentials/private data carry `Cache-Control: no-store`. CORS allows only the local development origins. Database URLs are not logged by startup failures.
- Runtime requires forced RLS on implemented private resources and rejects roles that own public tables (including inherited ownership), superusers, and BYPASSRLS roles. Migration credentials are never used by the API.
- Password changes, email verification, recovery, dashboard export/deletion controls, cleanup of expired sessions, TLS/ingress configuration, distributed rate limiting, and independent review remain launch prerequisites.

## Account export and deletion

`GET /api/v1/auth/export` returns a versioned JSON snapshot of the current account's product data and public profile fields. It excludes password/session digests and security/lockout metadata. Every exported resource has an explicit user predicate plus forced RLS. Repeatable-read isolation prevents mixing tracker/progress/rating revisions; transient serialization conflicts receive bounded retries. Responses are non-cacheable attachments.

`DELETE /api/v1/auth/account` accepts exactly `{ "password": "..." }` and requires a current bearer session and password reauthentication. The authentication hashing gate also covers deletion. The account is exclusively locked before session records, preventing cross-session lock-upgrade deadlocks. Deletion atomically cascades all sessions and user-owned database records. User IDs in request bodies are rejected. This is permanent; there is no dashboard deletion control or recovery flow yet. Shared catalog data remains intact.

## Remote session management

`GET /api/v1/auth/sessions` returns at most 100 active sessions: the current session first, followed by the newest sessions. It includes stable session IDs and timestamps, never token values or digests. The response reports `hasMore` when older active sessions are omitted. Revoked/expired sessions are excluded, and all reads use both an explicit owner predicate and forced RLS.

`DELETE /api/v1/auth/sessions/{id}` revokes a single owned active session, including the current session if explicitly targeted. Unknown, foreign, expired, or already-revoked IDs return the same 404. `POST /api/v1/auth/sessions/revoke-others` revokes every other active session, including any outside the listing limit, while keeping the caller authenticated. Its response reports the affected count; repeating it is a no-op.

Refresh now replaces the credential digest in the same session record and extends expiry to 24 hours from rotation. The previous token stops resolving, and the session ID/initial creation time stay stable. This prevents a concurrent refresh from escaping remote revocation aimed at the session ID. Revocation takes an exclusive account lock before session rows, while refresh/protected requests take a shared account lock. Race tests cover both orderings, mutual revocation, and cross-user attempts. No raw IP or user-agent data is collected.

The PostgreSQL dashboard provides an active-session panel with individual revocation and sign-out of all other sessions. It loads metadata on request, disables overlapping actions, refreshes after mutations, and removes the panel when the account signs out or becomes unauthorized. The current session uses the existing sign-out control. The same in-memory API client aborts and rejects stale requests when the account changes.
