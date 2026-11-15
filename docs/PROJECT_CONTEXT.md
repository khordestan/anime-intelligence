# Anime Intelligence — project context

Last updated: 2026-11-15. GitHub `main` is the source of truth. Update this document alongside implementation changes; use Git history and Actions for exact commit and validation status. This file contains no real user data or operational credentials.

## Goal and working agreement

Build a detailed, current anime catalog with personal tracking, explainable taste intelligence, and new episode/season alerts. Develop autonomously with actual code, tests and commits. Keep the repository public, fix failing checks before moving on, and do not deploy publicly before authentication, isolation and production security are ready. A ChatGPT Project may point here; it must not become a competing copy of implementation state.

## Current architecture

| Area | Implementation and boundary |
| --- | --- |
| API | Go 1.26 modular monolith, versioned REST under `/api/v1`; `cmd/api`, `internal/httpapi`, domain handlers in `internal/modules` |
| Persistence | Request-aware `store.Repository`; PostgreSQL via pgx for multi-user runtime, JSON file only for single-user local development |
| Database | Embedded checksummed transactional migrations `000001`–`000008`; advisory migration lock; separate administrator and restricted runtime roles |
| Identity | Argon2id passwords; random opaque bearer sessions; database stores digests; transaction-local identity and forced RLS on twelve private tables |
| Dashboard | Next.js 16 / React; in-memory bearer/proof handling, cancellation and account-revision checks prevent stale responses crossing accounts |
| Mail | Optional certificate-verified implicit-TLS SMTP; bounded pre-issuance queue; AES-256-GCM encrypted PostgreSQL outbox, leased claims and five persisted delivery attempts |
| Worker | `cmd/worker` is a lifecycle placeholder; the API currently owns identity delivery; no durable scheduler/provider sync exists |
| Intelligence | Deterministic, explainable pure-Go Personal Match core with tests; recommendation HTTP serving is pending |

PostgreSQL is selected by `DATABASE_URL`; connection/schema/security failures stop startup without falling back to JSON. Provision with `cmd/migrate` and `deployments/runtime-role.sql`. See [runtime guide](POSTGRES_RUNTIME.md), [architecture](ARCHITECTURE.md), [security contract](IDENTITY_SECURITY.md), and [roadmap](ROADMAP.md).

## Implementation status

- Implemented: public catalog search/detail; authenticated tracker create/list/update, basic episode progress and 1–5 ratings; verified email required for PostgreSQL tracker/rating access.
- Implemented: registration/login/logout, refresh with stable logical session IDs, session listing and remote revocation, password change, email verification, password recovery, consistent private export and password-confirmed deletion, with dashboard controls.
- Implemented: persisted password-failure window shared across login, password changes/deletion and API instances; ten failures in fifteen minutes cause a fifteen-minute lock. Success/password change/reset clears it; active locks are not extended.
- Implemented: canonical catalog/provenance schema, eight starter titles and forty-feature taxonomy. Schema coverage is broader than runtime feature coverage.
- Pending: licensed provider adapters, broad/current metadata, canonical entity resolution and field provenance, franchise/season/episode graph, richer tracking, release synchronization/alerts, preference profiles, recommendation serving, AI analyst and hybrid-provider integration.

## Key decisions and security constraints

- Never trust request-supplied user IDs. Resolve a bearer digest, acquire account state before session/proof rows, recheck credentials under locks, then run owner-scoped queries with transaction-local RLS context. RLS supplements application authorization.
- API database role must not own tables or have elevated privileges/BYPASSRLS. Migrations use separate credentials. Require migration 000008 at startup and reapply runtime-role grants.
- Sessions expire after 24 hours. Refresh changes the digest in the same row, preventing remote-revocation races. Password change/reset revokes all sessions; account deletion cascades private records.
- Only explicit bearer headers are supported; cookies/CSRF handling are not implemented. Browser reload drops the credential. Never persist credentials in browser storage.
- Unverified users retain account/recovery/export/deletion access; reset and refresh do not establish email ownership. Mail-disabled installations cannot unlock new accounts' product access.
- Recovery/verification proofs are random, purpose/address-bound, expiring and single use. Challenge records hold digests; a separate encrypted outbox temporarily holds the delivery envelope, including the proof, encrypted with a deployment key held outside PostgreSQL. Reset lasts thirty minutes; verification one hour. Mail links use fragments, scrubbed immediately, and explicit POST confirmation. Issuance has one-minute cooldown/five-per-hour quotas.
- Retries resend the same message without reissuing or consuming additional issuance quota. All errors/logs must redact provider responses, recipients and secrets. SMTP DATA acceptance ends retries; failed QUIT does not cause another send. Ambiguous network failures can still duplicate a message.
- Keep real account details, personal watch histories, secrets, database dumps and real credential hashes out of this public repository. Use synthetic fixtures only. Do not add personal preferences from chat to seed data.

Outbox claims use account → challenge → outbox lock ordering and release all database locks before SMTP. One-minute leases carry random hashed credentials; stale acknowledgements cannot complete a newer claim. Retry delays are 30/60/120/240 seconds, capped at five claims including crashes. Success, exhaustion, expiry or invalidation erases ciphertext; completed metadata is removed after one day when workers run. HTTP/export never exposes outbox rows.

## Known issues and next priorities

1. **Durable request acceptance and key operations:** issued challenges and encrypted mail now commit atomically and survive restart. HTTP 202 still acknowledges a volatile 32-item pre-issuance queue, so restart can lose accepted requests before issuance. Make that boundary durable without account enumeration or plaintext session/proof storage. Configure `MAIL_OUTBOX_KEY` consistently across instances, protect it separately from database backups, and drain old work before rotating it; key-ring rotation is pending. Wrong keys/corrupt envelopes preserve ciphertext and fail closed but can block the oldest pending job until corrected or expired. SMTP remains at-least-once; ambiguous failures may duplicate mail.
2. **Production abuse controls:** add reviewed ingress/distributed source throttling and monitoring. The process-wide password gate bounds hashing, but can be exhausted; per-account locks can deny service to known addresses. Do not treat these as complete public-ingress protection.
3. **Operations:** secret-free health/queue/security telemetry, expired-session/proof cleanup, backup/restore exercises, TLS/ingress and mail runbooks, load/reliability testing and independent security review. No public deployment is authorized yet.
4. **Product delivery:** provider capability/license review and adapters, season/episode relations and freshness, tracking completeness, then release alerts and explainable recommendation serving. Preserve provenance and spoiler scope.

## Verification and resuming work

1. Read root `AGENTS.md`, fetch GitHub and inspect `git status`, latest commits, this file, `ROADMAP.md`, `IMPLEMENTATION_LOG.md`, and current Actions/CodeQL results. Read any scoped `AGENTS.md` before editing (including `web/AGENTS.md`). Do not assume cached toolchains or databases exist in a fresh Work session.
2. Run `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`. Supply `TEST_DATABASE_URL` pointing to a disposable PostgreSQL 17 administrator connection for integration tests; without it database tests skip. Tests create isolated random databases/roles and exercise restricted runtime access.
3. For frontend changes use `npm ci`, `npm test`, `npm run typecheck`, `npm run build` in `web`; use the lockfile. CI also audits dependencies and runs Go vulnerability checks.
4. CI has backend, frontend/mocked-browser, database/migration/rollback, and two real PostgreSQL/browser suites (`postgres`, `identity`). Both use disposable restricted database roles and a loopback-only synthetic TLS mailbox. CodeQL runs separately. Verify checks for the exact pushed SHA; a local pass does not prove browser CI.
5. Commit implementation/tests and context updates together where practical. Record verified results in the implementation log; leave pending checks explicitly pending. Keep code and context in GitHub rather than a separate artifact store.

Latest verified implementation: `cb428e0` (encrypted durable identity delivery), with all five [CI jobs](https://github.com/khordestan/anime-intelligence/actions/runs/43699049145) and both [CodeQL analyses](https://github.com/khordestan/anime-intelligence/actions/runs/43699049052) passing. Local full PostgreSQL race tests, vet and build also passed. Root `AGENTS.md` directs future sessions to read/update this context. For later changes, inspect Actions for their exact SHA. Nothing has been publicly deployed.
