# Provider capability and approval registry

Reviewed 2026-11-15 against primary sources. No external provider is enabled by the application or database seed. Adapter code and API/software licenses do not establish a right to build a competing metadata/tracker product or redistribute source content.

| Source | Implemented input | Metadata | Relations | Episode schedules | Confirmed availability | Activation |
| --- | --- | --- | --- | --- | --- | --- |
| Reviewed neutral feed | Strict version-1 JSON | Rich structured title metadata | Typed canonical resolution | Revisioned by track | Explicit releasedAt | Administrator source registration required |
| Jikan v4 | Offline full-anime and episode-page converters | Names, aliases, synopsis, media/status/year/quarter, nullable totals, genres/themes/demographics/studios | Anime-only supported relations; manga/adaptation edges excluded | Aired timestamps as UNKNOWN-track schedules | Unsupported; never inferred | Network collection disabled pending source-use approval |
| AniList | None | Not ingested | Not ingested | Not ingested | Not ingested | Excluded under current tracker/collection restrictions |

## Jikan

Primary references: [OpenAPI source](https://github.com/jikan-me/jikan-rest/blob/master/storage/api-docs/api-docs.json), [API docs](https://docs.api.jikan.moe/), [service](https://jikan.moe/), [terms link](https://jikan.moe/terms).

The published specification describes a read-only unofficial MyAnimeList API, 24-hour caching and limits of 3 requests/second and 60/minute. Its software MIT license does not resolve underlying metadata, synopsis or image rights. The linked terms page could not be retrieved during this review; current redistribution/storage/attribution permissions remain unverified. No requests to catalog API endpoints or bulk collection were performed. Do not enable a source until those permissions and deployment usage are reviewed.

The converter accepts lawfully obtained offline response files:

```sh
go run ./cmd/catalog-convert -input=/path/to/full.json -observed-at=2026-01-02T00:00:00Z
go run ./cmd/catalog-convert -input=/path/to/full.json -episodes=/path/to/page1.json,/path/to/page2.json -observed-at=2026-01-02T00:00:00Z
```

Provide the actual observation time; conversion time does not make old data fresh. Inputs are bounded, trailing JSON rejected, selected typed fields normalized and arbitrary provider fields dropped. Episode pages must have consistent has_next_page flags for the supplied chain; duplicate episodes across pages fail final feed validation. This is not proof that every provider page or field is present. Unknown totals stay unknown. Unknown enum values map to UNKNOWN. Genre/theme names do not establish curated feature scores.

No network client, retries, cache, periodic job, source activation or public publishing is added. The command writes normalized JSON only to stdout, suitable for separate review and catalog-check/catalog-import. Aired dates, video links, finished status and scores never become confirmed availability events.

## AniList

[Official API terms](https://docs.anilist.co/guide/terms-of-use) restrict competing anime/manga tracker services and prohibit mass collection/storage use. This project is a tracker with a persistent catalog, so it must not assume permission. No AniList adapter or collection is enabled. A separate suitable agreement or a different approved source is needed for that integration.

## Remaining evidence and operations

Record approval scope, policy URL/review date, required attribution, permitted fields, cache/retention/deletion terms and quotas before enabling any source. Current data_sources.enabled plus policy_url is an administrator-controlled gate; it is not an automated license check. Scheduling, availability verification, region/language semantics and provider health telemetry remain pending.
