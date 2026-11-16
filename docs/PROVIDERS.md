# Provider capability and approval registry

Reviewed 2026-11-15 against primary sources. No scheduled external provider is enabled by the application or database seed. A bounded explicit-item Wikidata fetch command is implemented under its CC0 structured-data terms. Adapter code and API/software licenses do not establish a right to build a competing metadata/tracker product or redistribute source content.

| Source | Implemented input | Metadata | Relations | Episode schedules | Confirmed availability | Activation |
| --- | --- | --- | --- | --- | --- | --- |
| Reviewed neutral feed | Strict version-1 JSON | Rich structured title metadata | Typed canonical resolution | Revisioned by track | Explicit releasedAt | Administrator source registration required |
| Wikidata CC0 | Explicit-item live Action API fetch | Selected supported anime-work labels/native names/aliases, genres, year and non-conflicting episode totals | Unsupported | Unsupported | Unsupported | No scheduled job or database source enabled |
| Jikan v4 | Offline full-anime and episode-page converters | Typed/localized and legacy names, aliases, synopsis, media/status/year/quarter, nullable totals, separate genres/themes/demographics plus legacy tags/studios | Anime-only supported relations; manga/adaptation edges excluded | Aired timestamps as UNKNOWN-track schedules | Unsupported; never inferred | Network collection disabled pending source-use approval |
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

## Wikidata CC0 factual metadata

Primary references: [Data access and etiquette](https://www.wikidata.org/wiki/Wikidata:Data_access/en), [structured database license](https://www.wikidata.org/wiki/Wikidata:Database_download), [anime television series class Q63952888](https://www.wikidata.org/wiki/Q63952888). Structured entity facts are CC0; this does not extend to media assets, Wikipedia prose or other linked content. No prose/media is downloaded. Preserve Source: wikidata attribution in the UI as good provenance practice.

```sh
go run ./cmd/catalog-fetch -ids=Q124570859
```

The command requests up to 20 explicit Q IDs from the fixed HTTPS Action API endpoint. It requires a direct instance-of assertion from the reviewed class mapping below, rejecting unsupported/ambiguous works. Each imported item retains Wikidata's entity granularity: a series can span multiple seasons, while an explicitly classified season is a separately identified TV title. The adapter does not infer parent/season numbers, split series, traverse subclasses or crawl for titles. Labels/aliases, P136 genre labels, P577 publication year and P1113 dimensionless integer episode totals are normalized. Preferred ranks supersede normal ranks; deprecated claims are ignored; conflicting totals stay unknown. The earliest asserted publication year does not establish a precise broadcast date.

Requests include a project User-Agent, maxlag=5, bounded response sizes, 15-second request timeout, process-wide serialization and one-second pacing. Redirects are refused. HTTP 429/503 Retry-After advice is returned without raw provider content and retained as a process-local cooldown; delays longer than an hour are preserved. Oversized numeric values saturate at the maximum representable duration rather than becoming an immediate retry. No automatic retries occur inside a fetch. The opt-in worker additionally persists attempt/cooldown state. Genre labels are fetched in at most two batches. Overall command timeout is one minute.

A single primary API smoke check for Q124570859 returned HTTP 200 with the expected direct anime-TV class. No data was imported, user account read, media downloaded or source activated. Synthetic transport tests cover normalization, missing/unsupported entities, rank ambiguity, unknown facts, headers/pacing boundaries, redirects, rate limits, oversized/malformed responses and cancellation.

Review the normalized feed before import. An administrator can register provider_key=wikidata with the official data-access policy URL and enabled=true in their own database, then use catalog-import. No source activation is performed by catalog-fetch. Existing authoritative mappings are not overwritten or name-merged. Wikidata is a sparse factual fallback, not a comprehensive anime/availability feed: synopsis, studio claims, season graph, streaming rights, airing schedules and confirmed SUB/DUB releases remain unknown. The opt-in bounded worker and persisted source admission/cooldowns are implemented but idle by default. Durable per-title fetch jobs and persistent response caches remain pending.

## 2026-11-16 title schema compatibility

Rechecked the primary OpenAPI schema's current titles collection and deprecated legacy title fields. Offline conversion prefers typed Default and Japanese entries, retaining localized/unknown-type names as search aliases. Legacy title fields remain fallbacks and aliases. Without Default/legacy canonical data, English then Japanese provides a deterministic fallback; unclassified names alone do not become canonical identity. Conflicting nonblank Default/English/Japanese declarations fail closed. This changes no source-use permission or network activation.

The neutral feed and details API now preserve themes and demographics separately. Jikan conversion also keeps the prior combined tags for compatibility; category labels never become inferred user attributes or curated style scores. Sparse providers leave unsupported categories empty. The dashboard hides themes behind spoiler reveal and avoids repeating categorized labels in generic tags.

## 2026-11-16 reviewed explicit Wikidata media classes

| Direct P31 class | Canonical media |
| --- | --- |
| [Anime television series Q63952888](https://www.wikidata.org/wiki/Q63952888) | TV |
| [Anime television series season Q100269041](https://www.wikidata.org/wiki/Q100269041) | TV, preserving that entity identity |
| [Anime film Q20650540](https://www.wikidata.org/wiki/Q20650540) | MOVIE |
| [Original video animation Q220898](https://www.wikidata.org/wiki/Q220898) | OVA |
| [Original net animation Q1047299](https://www.wikidata.org/wiki/Q1047299) | ONA |

Explicit OVA/ONA formats take precedence over a co-asserted broad anime-film class. TV plus film/OVA/ONA, or OVA plus ONA, is ambiguous and rejected; TV series plus TV-season resolves to TV. Existing preferred/deprecated rank rules still apply. Generic film/anime classes or suggestive names alone are insufficient. No unsupported subclass closure, language/region release availability, exact season links or a default one-episode movie total is inferred. This review expands structured-fact mapping only; no catalog item or source was activated.
