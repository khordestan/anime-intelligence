# Followed schedule calendar

Verified PostgreSQL accounts can read GET /api/v1/calendar?from=2026-12-01&days=14. from is a UTC date (default today); days is 1–31 (default 14). The window includes its start and excludes its end. JSON demo mode has no calendar route.

The calendar projects only the caller's follows, including canonical PREQUEL/SEQUEL franchise expansion and selected release tracks. ALL includes every stored track; UNKNOWN remains distinct. Multiple matching follows/graph paths do not duplicate schedules. It selects the latest revision per title/episode/track before applying the window, so a rescheduled episode cannot revive an obsolete in-window date.

Only schedules with no recorded confirmed-release timestamp appear. Dates are predictions and can change; they do not establish availability in any region/language. The response includes public title/episode/track, source/revision and observation time when known, plus the actual from/to window. It does not expose owner/follow IDs, materialize notifications, send mail or activate a provider.

Results are ordered by scheduled time, title, canonical ID, episode and track. At most 100 are returned; truncated tells the client to narrow the window. Query statements have a two-second timeout. Verified identity, owner predicates, forced RLS and one consistent snapshot cover follow expansion and schedule selection. Serialization/deadlock conflicts retry at most three times with identity rechecks.

The dashboard provides explicit open/refresh, UTC start and 1/7/14/31-day choices, source/revision labels and an honest no-schedule state. It labels schedules as unconfirmed and keeps data only in memory. Account-keyed remounts and request guards fence late results. Existing runtime grants/schema suffice; no new migration or operational activation.

Comprehensive source coverage, regional availability, source freshness guarantees and background delivery remain separate work. Empty results can mean missing source data or unmatched track preferences, not a guarantee that no episode will air.
