# Catalog freshness diagnostics

catalog-status reads one repeatable-read, read-only database snapshot and emits public catalog aggregates. It never queries users, sessions, follows, progress or notifications and does not activate providers, mutate data, fetch the network or expose an HTTP endpoint.

```sh
go run ./cmd/catalog-status
go run ./cmd/catalog-status -stale-after=12h
```

Supply CATALOG_DATABASE_URL separately; DATABASE_URL is not a fallback. Prefer the optional deployments/catalog-reader-role.sql login with an out-of-band password. It has only SELECT on public source/state/metadata/anime/schedule tables and no private-table or write grants. Review an existing role before reusing its name; the script does not erase pre-existing privileges. Do not place credentials in tracked files.

Default freshness threshold is 24 hours, configurable from one minute to thirty days. The command has a ten-second deadline and fixed failure messages without connection strings or provider responses. Output includes:

- As-of time and threshold.
- Each source's provider key, enabled/policy-recorded flags, latest source observation, age, imported title count, never-synced/future-clock/stale flags.
- Last/next attempt times, bounded attempt count and allowlisted worker outcome when an attempt has been admitted.
- Total catalog titles, observed/unobserved title counts and latest episode schedules/confirmed episode counts, excluding old schedule revisions.
- Per-source oldest/newest title observations and stale/future title counts, plus catalog-wide totals. A title exactly at the freshness cutoff is current; future observations count as stale. Title age counts include disabled sources, independently of the enabled-source stale flag.
- Per-source and overall metadata coverage: titles with known episode totals (including zero), nonblank synopsis and nonempty synonyms/genres/themes/demographics/tags/studios/relations. These counts measure field presence, not correctness, licensing, resolution or comprehensive taxonomy coverage. Schedule counts do not certify streaming availability.

Enabled sources with missing policy records, no observation, future observations or excessive age are stale. Disabled sources do not count as stale even if they have never synced. Future ages clamp to zero and carry a separate invalid-clock flag. An observation timestamp certifies a successful import/fetch boundary, not the truth/completeness/freshness of every upstream field. The provider-wide timestamp can also reflect a partial feed; per-title aggregate age stays independent, so a fresh provider can still have stale titles. Unobserved starter titles are reported separately instead of being counted as current. Inspect per-title observedAt when assessing a specific title.

## Operator follow-through

Register/enable only reviewed sources. Wikidata CC0 explicit-item fetch is available for sparse TV-anime facts; Jikan source-use permissions remain unverified and AniList is excluded under current tracker/collection restrictions. No scheduler, host service or alert receiver is activated. Retention policies, provider-wide/per-title SLOs, metrics export, catalog writer-role hardening, availability verification and load tests remain work before public operation.

See [feed ingestion](CATALOG_FEEDS.md), [provider registry](PROVIDERS.md), [exact episodes](EPISODE_PROGRESS.md) and [in-app alerts](RELEASE_ALERTS.md). Do not expose the API publicly.

Migration 000014 and refreshed reader grants are required for attempt diagnostics. A stale source in a provider cooldown is not permission to retry early; inspect nextAttemptAt and respect it. No raw response/error strings or item-selection lists are persisted.
