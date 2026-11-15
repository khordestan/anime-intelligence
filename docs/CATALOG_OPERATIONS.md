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
- Total catalog titles and latest episode schedules/confirmed episodes by release track, excluding old schedule revisions.

Enabled sources with missing policy records, no observation, future observations or excessive age are stale. Disabled sources do not count as stale even if they have never synced. Future ages clamp to zero and carry a separate invalid-clock flag. An observation timestamp certifies a successful import/fetch boundary, not the truth/completeness/freshness of every upstream field. The provider-wide timestamp can also reflect a partial feed; inspect per-title observedAt when assessing a specific title.

## Operator follow-through

Register/enable only reviewed sources. Wikidata CC0 explicit-item fetch is available for sparse TV-anime facts; Jikan source-use permissions remain unverified and AniList is excluded under current tracker/collection restrictions. No scheduler, host service or alert receiver is activated. Retention policies, provider-wide/per-title SLOs, metrics export, catalog writer-role hardening, availability verification and load tests remain work before public operation.

See [feed ingestion](CATALOG_FEEDS.md), [provider registry](PROVIDERS.md), [exact episodes](EPISODE_PROGRESS.md) and [in-app alerts](RELEASE_ALERTS.md). Do not expose the API publicly.
