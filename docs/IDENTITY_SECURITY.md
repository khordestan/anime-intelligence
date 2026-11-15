# Identity and session security design

## Status

The database primitives in migration `000004_auth_sessions` are implemented and covered by PostgreSQL CI checks. This document describes the intended application contract. **Authentication is not implemented in the running API yet.** The local JSON API remains single-user development-only and must not be exposed publicly.

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
- Every user-owned read, update, delete, and feedback query must include the authenticated `user_id` in its predicate.
- Migration `000005_row_level_security` enables and forces RLS on users, sessions, watch history, ratings, progress, preferences, recommendations, feedback, follows, and notifications. Policies use the transaction-scoped `app.user_id` setting. The API does not set this context yet, so PostgreSQL runtime access is not ready. The application database role must not be a superuser or have `BYPASSRLS`; do not run the application as the schema owner.
- The `app.user_id` setting is a trusted server-side context, not an authentication mechanism. Only set it after verifying a server-side session, within the same transaction as the protected queries. Never derive it from request input.
- Test cross-user access attempts for every user-owned resource, not only happy paths.
- Keep catalog data public/shared; isolate watch history, ratings, notes, preferences, recommendation feedback, follows, and notifications.

## Abuse controls

- Add rate limits for login, registration, password reset, and token verification.
- Use generic login errors to avoid account enumeration.
- Apply progressive throttling/temporary lockout carefully; do not allow trivial denial-of-service against a known account.
- Emit security audit events without credentials or sensitive request bodies.
- Provide session listing/revocation and account data export/deletion before public launch.

## Public repository rules

Only schema, source code, synthetic fixtures, and documentation belong in this public repository. Real account details, personal anime histories, session tokens, password hashes from real accounts, production database dumps, and secrets must stay out of Git history.
