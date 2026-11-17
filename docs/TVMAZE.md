# Reviewed TVmaze anime seasons

Primary sources reviewed 2026-11-17: [TVmaze API](https://www.tvmaze.com/api), [copyright policy](https://www.tvmaze.com/site/copyright), and the API's linked [CC BY-SA 4.0 license](https://creativecommons.org/licenses/by-sa/4.0/). TVmaze permits reuse subject to attribution and ShareAlike. The public service documents one-hour response caching and at least 20 requests per ten seconds per IP; this adapter serializes requests at one per second and backs off on 429/503. Cached API observations may lag edits.

TVmaze-derived normalized data is adapted under CC BY-SA 4.0. Preserve TVmaze credit, the license link and the adaptation notice when distributing that data. The dashboard includes these links in catalog details and its footer. Downstream exports/publications must retain the same notices.

## Explicit identity and scope

Use IDs shaped **show-101-season-501**, with positive numeric TVmaze show and season IDs (up to nine digits each). These examples are synthetic. An operator must review that the work is anime and the season corresponds to the intended canonical title. Animation alone is insufficient. No search, catalog crawl, automatic genre/country classifier, name merge or franchise link is implemented.

Each fetch requests exactly three fixed HTTPS paths: show details, that show's season list, then the selected season's episodes. The selected season must appear in the show's returned list, and its episode season numbers must agree. Provider-returned links are never followed. Redirects are rejected; responses are individually bounded at 8 MiB, requests at fifteen seconds and worker cycles at two minutes. Transport/body errors are fixed messages, without raw responses or URLs.

Canonical identity retains the full show/season pair. The display name combines the provider show name and explicit season number; this is an adaptation label, not a provider-supplied season title. Only TV media type and an explicit season premiere year are retained alongside regular episode schedules. Season status, total episodes, language track, synopsis, artwork, show-level genres and season relations remain unknown. Planned episode orders and counts of returned episodes do not establish final totals.

Specials and unknown episode types are excluded; regular episode numbers retain gaps and are never flattened across seasons. Missing airstamps stay unknown. Both past and future airstamps are scheduledAt with UNKNOWN track, never releasedAt. Neither an ended show nor a past airdate confirms a region/language release. Imported schedules therefore cannot create confirmed episode-release notices. Existing canonical authority conflicts still fail closed; no source authority is silently transferred.

## Reviewable command

```sh
go run ./cmd/catalog-fetch -provider=tvmaze -ids=show-101-season-501 -reviewed-anime-seasons
```

The command writes a neutral snapshot to stdout. Review it before catalog-check/catalog-import, and preserve this source/license notice with distributed results. It does not connect to a database, enable a source, or activate a host process. Only one show/season pair is accepted per invocation; default Wikidata command behavior is preserved.

## Opt-in durable worker

Apply migration **000017** (including 000016's jobs) and retain the catalog-writer grants from 000016. Existing API grants are unchanged. Independently register an approved data_sources row with provider_key=tvmaze, an appropriate reviewed policy_url and enabled=true. The worker cannot change those fields.

Set CATALOG_SYNC_PROVIDER=tvmaze, CATALOG_SYNC_IDS to up to twenty reviewed show/season pairs, and CATALOG_SYNC_REVIEWED_ANIME_SEASONS=1. CATALOG_DATABASE_URL must use the dedicated writer; CATALOG_SYNC_INTERVAL remains the per-title refresh interval, one hour to seven days. Missing IDs leave the worker completely idle. Wikidata remains the default provider.

One provider is configured per process. Different processes may serve each supported provider on the same database; the shared collector lock serializes fetch cycles, while due times, jobs, high-water marks and cooldowns remain source-specific. The existing lease/retry/atomic-import contract applies unchanged. A TVmaze Retry-After is persisted across restarts and newly added TVmaze IDs without delaying Wikidata's source state.

Migration 000017 rollback fails atomically if TVmaze jobs remain. Operators must explicitly review/archive those retained public jobs before downgrading; rollback never silently deletes queued work.

No live source, public endpoint collection, database source row or host process was activated during implementation. This adapter covers reviewed season facts and schedules only. Broad discovery, comprehensive metadata, cross-provider field arbitration and confirmed availability feeds remain unfinished.
