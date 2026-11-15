# Confirmed episode alerts

Migration 000012 adds per-follow track preferences and durable per-user/event/channel notification uniqueness. Reapply deployments/runtime-role.sql before starting the updated API. All private operations require a valid bearer session and verified email, use explicit owner predicates and run under existing forced RLS. No client user ID is accepted.

## API

- PUT /api/v1/follows/{animeId} with {"track":"SUB"} subscribes; RAW, DUB, UNKNOWN and ALL are also supported.
- GET /api/v1/follows returns up to 1000 owned subscriptions.
- DELETE /api/v1/follows/{animeId} removes only the caller's subscription.
- GET /api/v1/notifications materializes up to 100 eligible events and returns the newest 100 owned in-app notices.
- PATCH /api/v1/notifications/{id}/read marks an owned notice read idempotently. Foreign and missing IDs both return 404.

Only administrator-imported EPISODE_RELEASED events can become automatic notices. Elapsed schedules are not events. Track preferences must match, or be ALL. Subscriptions start at their creation time; earlier releases do not replay. Updating a track retains that time, allowing newly selected tracks since the original follow. Removing/recreating a follow begins a new subscription. Removing a follow does not delete existing notices.

Materialization occurs on authenticated polling; it is not background mail, push or guaranteed immediate delivery. It has no cross-account worker identity or RLS bypass. Unique notification keys make retries and concurrent sessions safe. Account export includes subscriptions/preferences and notices through existing resources; account deletion removes them through existing cascades.

## Remaining work

Dashboard controls/polling, pagination/history beyond the response limits, franchise follows/new-season events, region/language availability and background delivery channels remain pending. Network source synchronization is not enabled. Ingestion still requires explicit reviewed offline snapshots. Public deployment remains unauthorized.
