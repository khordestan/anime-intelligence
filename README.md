## Current implementation

- Go HTTP API: health, catalog search/detail, tracker create/list/update, progress, and 1–5 ratings.
- Next.js dashboard: starter-catalog search, add-to-library, status filters, episode progress, and ratings.
- Local JSON persistence for the runnable single-user development slice.
- PostgreSQL canonical schema with users, catalog, source provenance, feature observations, tracker, ratings, preferences, recommendations, schedules, events, and notifications.
- Authentication/security schema foundation: required case-insensitive unique email, password-hash storage slot, account lock/disable metadata, hashed opaque session records, and forced PostgreSQL RLS policies (migrations 000004–000005). This is schema only; no login/session API or trusted per-request database identity context is wired.
- Deterministic Personal Match ranking core with separate explicit/inferred preferences, evidence output, exclusion filters, and unit tests (not yet exposed through the API).
- Provider-neutral catalog fixtures and a curated 40-feature taxonomy.
- GitHub Actions CI for Go tests, race detector, vet/build, frontend typecheck/build, PostgreSQL migrations/rollback, and auth schema constraints.
- Public-repository security baseline: CodeQL, dependency review, Dependabot, and contribution/security policies.

## Development status and limitations

This is a development foundation, not a production-ready multi-user service. The running API currently uses a local JSON store; PostgreSQL migrations are authored but are not wired into the API runtime. Authentication, sessions, runtime authorization/RLS context wiring, external provider synchronization, recommendation serving, AI analysis, and release notifications are not implemented.

**Do not expose the API publicly or use it for sensitive data.** The current local JSON store is single-user development infrastructure, not a secure account system.
