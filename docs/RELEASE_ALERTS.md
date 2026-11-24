# Confirmed episode alerts

Migration 000012 adds per-follow track preferences and durable per-user/event/channel notification uniqueness. Reapply deployments/runtime-role.sql before starting the updated API. All private operations require a valid bearer session and verified email, use explicit owner predicates and run under existing forced RLS. No client user ID is accepted.

## API

- PUT /api/v1/follows/{animeId} with {"track":"SUB"} subscribes; RAW, DUB, UNKNOWN and ALL are also supported.
- GET /api/v1/follows returns up to 1000 owned subscriptions by default. Use `?limit=50` to opt into paging with a limit from 1 to 100. Pass `nextCursor` as `cursor` to continue, and retain `limit` if you chose a page size other than 50. Pages sort by newest subscription time and then follow UUID. A cursor provides a boundary within the caller's own follows.
- GET /api/v1/follows/{animeId} returns the caller's subscription for one title. A title without an owned subscription returns 404.
- DELETE /api/v1/follows/{animeId} removes only the caller's subscription.
- GET /api/v1/notifications returns a materialization object with created (notices inserted by this poll, 0–100) and hasMore (another eligible event was observed beyond that batch). It materializes up to 100 eligible events and returns the newest 100 owned in-app notices with nextCursor when older materialized notices exist. GET /api/v1/notifications?cursor=... returns the next history page without materializing new events and omits materialization because it does not check the backlog. Invalid cursors return 400; cursors select a time/ID boundary and never an account. Every page rechecks the authenticated verified owner.
- PATCH /api/v1/notifications/{id}/read marks an owned notice read idempotently. Foreign and missing IDs both return 404.

Administrator-imported release and eligible linked-season announcement events can become automatic notices. Elapsed schedules create no notices. A newly ingested confirmed first release for a title classified as a movie is labeled Movie released. The classification is captured with the event, and a later format correction cannot relabel it. Older release events without a recorded format keep the episode label. Track preferences must match, or be ALL. Subscriptions start at their creation time; earlier releases do not replay. Updating a track retains that time, allowing newly selected tracks since the original follow. Removing/recreating a follow begins a new subscription. Removing a follow does not delete existing notices.

Materialization occurs on authenticated polling; it is not background mail, push or guaranteed immediate delivery. It has no cross-account worker identity or RLS bypass. Unique notification keys make retries and concurrent sessions safe. Account export includes subscriptions/preferences and notices through existing resources; account deletion removes them through existing cascades.

## Dashboard

Verified accounts can open Manage release alerts, subscribe library titles, select a release track, stop alerts and mark notices read. The visible dashboard polls every minute while viewing the latest page, including while the panel is collapsed. Collapsed polls request notification state only and show the unread total beside the panel button. Opening the panel refreshes the current subscriptions page and inbox. Subscriptions use pages of up to 50. Newest follows and Older follows navigate the subscriptions without polling the inbox. With multiple pages, choose a library title to read and manage its current subscription even when it falls outside the displayed page. The panel has a manual refresh button. Older notifications opens one history page at a time; automatic refresh pauses there until Back to latest alerts is selected. Failed page requests preserve the current page and can be retried. Marking a historical notice read preserves the page. It resets on account changes and rejects stale results. Local mode and unverified accounts do not expose the panel. Credentials and notices are never stored in browser storage.

## Remaining work

Operator backlog metrics, franchise curation/retractions, region/language availability and background delivery channels remain pending. Network source synchronization is not enabled. Ingestion still requires explicit reviewed offline snapshots. Public deployment remains unauthorized.

## Linked-season follows

PUT follows now accepts scope=ANIME (default, this title/season) or FRANCHISE (all canonical titles connected through PREQUEL/SEQUEL relations). Changing scope replaces the previous scope for that title while retaining subscription time. DELETE accepts a matching scope query parameter; it defaults to ANIME for compatibility. The dashboard exposes This season / All linked seasons.

After a provider's initial baseline, a newly observed ANNOUNCED title (or UNKNOWN-to-ANNOUNCED transition) creates one durable canonical SEASON_ANNOUNCED event. First provider baselines do not generate announcement floods. Franchise followers receive eligible linked announcements and confirmed matching-track episode releases, including transitive sequels. Single-season followers do not expand scope. Other relation types do not establish franchise membership.

Polling deduplicates events reachable through multiple follows/paths. Per-account advisory locks serialize subscription changes/materialization after account/session locks. A two-second database statement timeout bounds polling query work. An account may follow up to 1000 titles; existing track/scope preferences can still change at that limit. Notification times reflect event occurrence.

Graph corrections can change membership; historical notices remain. Announcement text reflects source claims, not guaranteed production or release dates. Explicit franchise curation, cancellations and production-scale performance evaluation remain pending. A synthetic PostgreSQL fixture exercises 80 overlapping follows through a 128-title chain. Calendar traversal coalesces equivalent title/track paths, and alert matching uses the earliest qualifying follow and consent boundaries per reachable title, track and scope.

See [followed schedule calendar](RELEASE_CALENDAR.md) for a separate owner-scoped view of upcoming unconfirmed schedules. Calendar reads never create or confirm release events.


## Catching up after a large event batch

The latest-page query selects at most 101 distinct eligible events in occurrence-time/ID order and inserts at most 100. Insertion and materialization status share that single statement under the existing owner/advisory locks and transaction. The extra candidate sets hasMore; overlapping follow paths cannot inflate it because candidates are distinct events. Existing notices, nonmatching tracks, unrelated titles and pre-subscription events are excluded before the cap.

hasMore describes eligibility at this poll's query snapshot, not an exact backlog count or a promise about later arrivals. created counts successful new inserts, not unread/returned notices. nextCursor continues to mean older already materialized history; it is independent of pending events. If another eligible event exists, `oldestPendingAt` is its occurrence time. It reflects the first event beyond the batch from that snapshot. History pages omit materialization and never drain the backlog. All status is owner scoped and verified-email protected.

The dashboard displays a catch-up notice on the latest page when hasMore is true. Manual Refresh alerts or the existing one-minute polling continues with another bounded batch. It does not loop immediately or create a background delivery channel. Successful polls replace the status; failed refreshes preserve the previous page/status for retry. History hides the catch-up notice and account changes reset it.

## Marking all alerts read

PATCH /api/v1/notifications/read-all marks the caller's existing unread in-app notices read across all pages and returns the number updated. Repeating it after all existing notices are read returns zero. The endpoint only updates materialized notices. The dashboard refreshes the current page after the action.

## Unread notifications

Use GET /api/v1/notifications?unreadOnly=true for an unread-only view across all pages. Keep the filter when requesting nextCursor. The latest page still checks for new release events. History pages only read stored notifications. The dashboard starts at the latest page when changing the filter and keeps the filter during refresh and pagination.

Notification pages include `unreadCount`, the number of the caller's unread in-app notices across all materialized pages. It is computed from the same database snapshot as the page, regardless of filters or cursors, and excludes release events awaiting materialization.

The dashboard pauses scheduled polling while its tab is hidden or a history page is selected. This delivers badges while the dashboard is visible. Delivery when the app is closed still requires a background channel.

GET /api/v1/notifications/summary performs the same bounded latest-page materialization and returns only the unread total and materialization status. Collapsed dashboard polling uses this endpoint and does not download individual notifications.

## Opt into schedule changes

Schedule notices are off for new follows. Select **Schedule changes** and save the release settings to enable them for an existing or new follow. The follow API exposes `scheduleNotices` and preserves existing consent when an update omits it. Changing track or scope starts a new schedule notice window. Clearing consent stops new notices and preserves previously materialized history. Verified email and owner scope apply.

Migration 000024 adds the consent timestamp. The upgrade keeps existing follows opted out. Rollback refuses to discard retained consent. Schedule notices still use the per-poll batch cap and the existing notification ledger. They state whether a published schedule moved earlier or later and include the before and after times and source. A change is an unconfirmed schedule revision.

Enable source publication only for a reviewed offline snapshot with permission for this notice use. Pass `-schedule-change-events` to `catalog-import`, with `-preview` for an atomic rehearsal. Two known observed schedules on the same title, episode and track must differ, and the release must still be unconfirmed. A baseline, newly known time, removed time, unchanged time or confirmed release creates no schedule notice event. Events use the snapshot observation time and canonical revision key. Runtime polling checks the later of follow start and schedule notice consent, so a new consent does not replay earlier moves. Queued providers do not enable this option.
