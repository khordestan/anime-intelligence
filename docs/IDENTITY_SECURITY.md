# Identity and session security design

## Status

PostgreSQL mode implements registration, login, session resolution, logout, atomic token rotation, and user-scoped tracker persistence. CI executes these flows through a restricted PostgreSQL role, with direct RLS and HTTP isolation assertions. The JSON API remains unauthenticated and single-user. Neither mode is approved for public deployment; production abuse controls, operational mail configuration, queue monitoring, inbox/outbox key operations and independent review remain pending. See [runtime setup](POSTGRES_RUNTIME.md).

The current API uses explicit `Authorization: Bearer` headers, never cookies. The browser dashboard supports PostgreSQL login, registration, logout, and expiry, with tokens held only in component-scoped memory. Changing accounts aborts pending requests and rejects late results; reload clears the client credential and private library. The server session remains subject to its 24-hour expiry unless explicitly revoked. Raw tokens must remain in volatile client memory, never URLs, logs, or browser persistent storage.

## Credential storage

- Email addresses are required and unique case-insensitively.
- Passwords must be hashed in the application with Argon2id using parameters reviewed against current OWASP guidance and the deployment's resource budget.
- Store only the encoded Argon2id PHC string in `users.password_hash`; never store plaintext passwords or reversible encryption.
- Password verification must use a vetted library and constant-time comparison as provided by that library.
- Keep password reset and email verification tokens out of logs; persist only token digests with expiry and one-time-use semantics, as implemented by the challenge flows.

## Session contract

- Generate at least 256 bits of cryptographically secure random entropy for each opaque bearer token.
- Return the raw token once to the client; persist only its SHA-256 digest in `auth_sessions.token_hash`.
- Enforce expiry and revocation on every authenticated request. Update last-used metadata with a bounded cadence to avoid a write on every request.
- Revoke sessions on logout, password change/reset, account disablement, and relevant security events.
- Current browser authentication uses explicit bearer headers only. Any future cookie implementation must use secure, HttpOnly cookies with an appropriate SameSite policy and CSRF protection for state-changing requests. Never put session bearer tokens in URLs or logs.
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
- Session listing, individual remote revocation, and revocation of other sessions are implemented through the PostgreSQL API. Account data export/deletion are implemented through the PostgreSQL API and signed-in dashboard.

## Public repository rules

Only schema, source code, synthetic fixtures, and documentation belong in this public repository. Real account details, personal anime histories, session tokens, password hashes from real accounts, production database dumps, and secrets must stay out of Git history.

## Implemented limits and remaining launch work

- Argon2id v19, 64 MiB, three iterations, parallelism two, 16-byte random salt, 32-byte output. Verification accepts only this bounded profile. Password input is 12–1024 bytes when registering.
- Authentication request bodies are limited to 4 KiB and reject unknown fields and trailing JSON. Login failures use generic responses, and missing accounts perform dummy hash verification.
- Registration/login, account deletion, and password changes share a process-wide burst of ten attempts, replenished at one attempt per six seconds, with at most two password operations in flight. This bounds memory use but does not replace distributed/per-account production controls; a global limiter can be exhausted by an attacker.
- Password failures also use a persisted per-account observation window and bounded temporary lock (migration 000007); login, password changes and deletion share this state across API instances.
- Sessions expire after 24 hours. Refresh atomically revokes the old token and issues a new one. Disabled/locked accounts are checked on every private transaction. Last-used writes are limited to a five-minute cadence.
- API responses containing credentials/private data carry `Cache-Control: no-store`. CORS allows only the local development origins. Database URLs are not logged by startup failures.
- Runtime requires forced RLS on implemented private resources and rejects roles that own public tables (including inherited ownership), superusers, and BYPASSRLS roles. Migration credentials are never used by the API.
- Operational mail/key configuration, queue monitoring, cleanup of expired sessions, TLS/ingress configuration, distributed rate limiting, and independent review remain launch prerequisites.

## Account export and deletion

`GET /api/v1/auth/export` returns a versioned JSON snapshot of the current account's product data and public profile fields. It excludes password/session digests and security/lockout metadata. Every exported resource has an explicit user predicate plus forced RLS. Repeatable-read isolation prevents mixing tracker/progress/rating revisions; transient serialization conflicts receive bounded retries. Responses are non-cacheable attachments.

`DELETE /api/v1/auth/account` accepts exactly `{ "password": "..." }` and requires a current bearer session and password reauthentication. The authentication hashing gate also covers deletion. The account is exclusively locked before session records, preventing cross-session lock-upgrade deadlocks. Deletion atomically cascades all sessions and user-owned database records. User IDs in request bodies are rejected. This is permanent. The dashboard requires the current password and an explicit acknowledgement, clears the password and acknowledgement after a failed attempt, and clears the session/library after success. Incorrect password confirmation returns `invalid_current_password` without invalidating a valid session. Shared catalog data remains intact.

## Remote session management

`GET /api/v1/auth/sessions` returns at most 100 active sessions: the current session first, followed by the newest sessions. It includes stable session IDs and timestamps, never token values or digests. The response reports `hasMore` when older active sessions are omitted. Revoked/expired sessions are excluded, and all reads use both an explicit owner predicate and forced RLS.

`DELETE /api/v1/auth/sessions/{id}` revokes a single owned active session, including the current session if explicitly targeted. Unknown, foreign, expired, or already-revoked IDs return the same 404. `POST /api/v1/auth/sessions/revoke-others` revokes every other active session, including any outside the listing limit, while keeping the caller authenticated. Its response reports the affected count; repeating it is a no-op.

Refresh now replaces the credential digest in the same session record and extends expiry to 24 hours from rotation. The previous token stops resolving, and the session ID/initial creation time stay stable. This prevents a concurrent refresh from escaping remote revocation aimed at the session ID. Revocation takes an exclusive account lock before session rows, while refresh/protected requests take a shared account lock. Race tests cover both orderings, mutual revocation, and cross-user attempts. No raw IP or user-agent data is collected.

The PostgreSQL dashboard provides an active-session panel with individual revocation and sign-out of all other sessions. It loads metadata on request, disables overlapping actions, refreshes after mutations, and removes the panel when the account signs out or becomes unauthorized. The current session uses the existing sign-out control. The same in-memory API client aborts and rejects stale requests when the account changes.

## Authenticated password changes

`POST /api/v1/auth/password` accepts exactly `currentPassword` and `newPassword`. It requires a verified bearer session, checks the current password, and applies the same bounded Argon2id policy to the replacement. The account is exclusively locked before session rows. The password hash update and revocation of every existing session, including the caller, commit atomically. The user must sign in again; the response does not issue a replacement credential.

Wrong current passwords return 401 with the allowlisted `invalid_current_password` code without modifying the account or revoking sessions. Invalid replacements return 400. The dashboard handles these reauthentication errors locally, clears password inputs, and preserves the valid session. Other 401 responses still clear private UI and request a new login. Late error-body decoding is subject to the same account-revision checks as successful responses. Concurrent password changes serialize; only one can succeed using the old credentials. Forgotten-password recovery and email verification are implemented separately through purpose-bound challenges and configured out-of-band delivery.

## Recovery delivery and browser proof handling

Migration 000006 adds a separate forced-RLS credential table. Digests are domain-separated by purpose and bound to the current stored email. Reset tokens last 30 minutes; verification tokens last one hour. Issuance holds an exclusive account lock, supersedes previous pending proofs after the resend cooldown, and caps issuance at five per account/purpose/hour. Password changes and resets invalidate all pending proofs; deletion cascades them. Redemption acquires account state before the proof row, atomically checks expiry/revocation/consumption/address binding, and succeeds only once under races. Reset clears temporary login locks and revokes sessions, but does not silently mark email verified or authenticate.

Recovery request acceptance is durable and generic. A fixed-size encrypted inbox commits before HTTP 202 without account lookup; configured runtime has no volatile fallback. Account lookup/issuance and SMTP run in independent workers. Inbox completion, proof issuance and encrypted mail commit together. Key operations and provider monitoring remain launch work. Delivery requires explicit SMTP credentials, verified implicit TLS, and a trusted configured app origin. Errors never include provider responses, email addresses, raw proofs, or credentials in logs. Default runtime has delivery disabled and advertises that capability to the dashboard.

Mail links use fragments, not query parameters. The dashboard immediately replaces the history entry without the fragment, keeps proof only in memory, and waits for an explicit confirmation POST. Unknown/repeated fields or malformed/ambiguous purposes fail parsing. Browser reload discards proof; links must be reopened if still valid. UI requests use the same account-revision checks as other operations, so account switches abort stale proof responses. PostgreSQL tracker and rating routes require email verification, including direct repository calls. Account and recovery controls remain accessible before verification.

The browser mail suite uses an explicitly enabled, loopback-only synthetic SMTP/HTTP fixture, a per-run certificate trusted by the client, and random masked test credentials. The fixture is not linked into the API image and rejects unauthenticated mailbox reads. It sends no external mail and must never handle real user accounts. Reference: [OWASP recovery guidance](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html).


## Verified-email product authorization

PostgreSQL registration creates an authenticated account with `emailVerified=false`. Every tracker read/create/update and rating operation requires `email_verified_at` to be set. The HTTP boundary rejects unverified sessions with 403 `email_verification_required` before processing product bodies. Repository operations independently check current account state under the same transaction/account lock used for data access. A captured user object or a freshly rotated session cannot bypass this check. Ownership predicates and forced RLS still apply after verification.

Verification is required even when SMTP is disabled. Missing delivery configuration must never weaken authorization: new/unverified accounts stay locked out of the library. Existing accounts are not automatically grandfathered into verification. Configure the trusted SMTP flow and prove address ownership; do not mark real accounts verified by modifying the database.

Unverified accounts retain `/auth/me`, login/logout/refresh, session controls, password change/recovery, verification requests/redemption, data export and password-confirmed deletion. Public catalog reads remain available. Password reset alone does not set email verification. After redeeming a verification proof, the existing session gains product access on its next request. The dashboard refreshes the current profile without persisting credentials, explains verification or delivery unavailability, and disables library editing while unverified. JSON mode remains single-user, unauthenticated development only.


## Login failure observation and temporary locks

Migration 000007 adds `login_failure_started_at`. The application records credentialed-account failures after hashing and rechecking the same credential/enabled/unlocked state under an exclusive account lock. Failure recording commits before a generic authentication denial is returned; database errors fail closed. Ten failed guesses within fifteen minutes trigger a fifteen-minute lock. The observation window resets after expiry, successful login, password change or password recovery. An active lock is not extended by either correct or incorrect login requests. No lock state, remaining attempts or existence information is returned to callers.

Known/unknown/disabled/locked accounts still undergo password or dummy-hash verification within the process-wide resource gate. A credential changed during verification cannot issue a session or charge the new credential's attempt window based on a stale hash. Current profile fields are read again under the lock before session issuance. Existing sessions are rejected while the account is locked, matching the established account-state policy. Recovery issuance/redemption deliberately remains available to an address owner during a login lock.

Account locks introduce a targeted denial-of-access tradeoff. These thresholds are a conservative development policy, not a completed production anti-abuse system. Source-aware ingress limits, distributed resource budgets, monitoring, operational delivery and independent review remain prerequisites for launch. Reference: [OWASP authentication guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html).


Incorrect current-password confirmations in `POST /auth/password` and `DELETE /auth/account` charge the same persisted failure window as login. Their already-held exclusive account locks serialize the check and budget update; failures commit before returning `invalid_current_password`, so transaction rollback cannot erase guesses. Before the threshold the session remains valid; reaching it causes the established locked-account session rejection on subsequent protected requests. Correct password changes clear the shared state while revoking all old sessions. Database errors never become successful reauthentication. This closes cross-instance guessing through account controls while retaining the same operational lockout tradeoff.


### Durable delivery security boundary

Migration 000008 adds a forced-RLS outbox with an encrypted envelope, never plaintext tokens. AES-256-GCM binds the envelope to its immutable challenge ID/version. `MAIL_OUTBOX_KEY` is an explicit deployment secret held outside PostgreSQL and must be shared consistently across replicas. Challenge issuance/outbox insertion commit together; a failed insert rolls back issuance. Outbox records are never returned by HTTP or private data export.

Trusted worker code uses transaction-local `app.delivery_worker` only to select encrypted delivery candidates and acknowledge lease credentials; it grants no global access to user/challenge records. After selecting an owner, worker context is cleared, account state is locked before the proof/outbox, and live purpose/address/digest/expiry/revocation are checked. No database locks remain during SMTP. One-minute leases are random and stored only as digests; acknowledgements must match the current lease. Five claims and persisted backoff bound retries, including crashes. Revoked/consumed/disabled/expired jobs are discarded on inspection; deletion cascades them. Credential invalidation after a claim can still produce an unusable email, but redemption always checks current account/proof state.

Success, expiry or exhaustion erases encrypted credentials, and workers purge completed metadata after one day. Wrong keys or corrupt envelopes fail closed while preserving work; they may block the oldest pending job until corrected or expired. Key-ring rotation and encrypted-backup/key operations need review before production. SMTP acceptance/acknowledgement ambiguity can duplicate mail, while proofs remain single use. HTTP acceptance uses the durable request inbox described below; production monitoring remains open.

### Request inbox security boundary

Migration 000009 adds a separate forced-RLS request inbox. Only trusted `app.identity_request_worker` code accesses it, and connection/transaction/identity transitions clear that context. Anonymous reset admission performs no account lookup; fixed-size 540-byte ciphertext and shared capacity decisions do not depend on account existence. A global admission advisory lock limits active intents to 256 across instances. Database errors and full capacity never return 202 or recipient-specific errors.

The encrypted request domain is distinct from mail envelopes and binds the immutable row ID. Verification stores only an encrypted session digest; it is resolved through the same live account/session locks and checks as bearer authentication. No API accepts a digest as a credential. A random hashed lease fences stale processing, while request completion, issuance and outbox insertion share one transaction. Account/session locks precede request and challenge/outbox locks; admission/claim/cleanup transactions never acquire account locks. Raw email/proof/session values must never enter logs.

Requests have a fifteen-minute deadline, five claims and bounded persisted backoff. Wrong keys/corruption preserve ciphertext and fail closed; expired/exhausted/completed payloads are erased, with completed metadata retained at most one day while workers run. Anonymous request intents cannot cascade by account because acceptance has no owner lookup; account deletion can leave encrypted reset intent until processing/expiry, which must always recheck live account state. Key backup/rotation, backlog monitoring, distributed ingress controls and independent security review remain required before public deployment.
