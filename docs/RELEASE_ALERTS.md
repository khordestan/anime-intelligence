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

## Dashboard

Verified accounts can open Manage release alerts, subscribe library titles, select a release track, stop alerts and mark notices read. The panel polls every minute while open and has a manual refresh button. It resets on account changes and rejects stale results. Local mode and unverified accounts do not expose the panel. Credentials and notices are never stored in browser storage.

## Remaining work

Pagination/history beyond the response limits, franchise curation/retractions, region/language availability and background delivery channels remain pending. Network source synchronization is not enabled. Ingestion still requires explicit reviewed offline snapshots. Public deployment remains unauthorized.

## Linked-season follows

PUT follows now accepts scope=ANIME (default, this title/season) or FRANCHISE (all canonical titles connected through PREQUEL/SEQUEL relations). Changing scope replaces the previous scope for that title while retaining subscription time. DELETE accepts a matching scope query parameter; it defaults to ANIME for compatibility. The dashboard exposes This season / All linked seasons.

After a provider's initial baseline, a newly observed ANNOUNCED title (or UNKNOWN-to-ANNOUNCED transition) creates one durable canonical SEASON_ANNOUNCED event. First provider baselines do not generate announcement floods. Franchise followers receive eligible linked announcements and confirmed matching-track episode releases, including transitive sequels. Single-season followers do not expand scope. Other relation types do not establish franchise membership.

Polling deduplicates events reachable through multiple follows/paths. Per-account advisory locks serialize subscription changes/materialization after account/session locks. A two-second database statement timeout bounds polling query work. An account may follow up to 1000 titles; existing track/scope preferences can still change at that limit. Notification times reflect event occurrence.

Graph corrections can change membership; historical notices remain. Announcement text reflects source claims, not guaranteed production or release dates. Explicit franchise curation, cancellations/retractions and measured large-graph performance remain pending.
