# Public catalog filters

GET /api/v1/anime supports the existing q, limit and offset parameters plus:

| Parameter | Meaning |
| --- | --- |
| mediaType | TV, MOVIE, OVA, ONA, SPECIAL, MUSIC, UNKNOWN |
| status | UNKNOWN, ANNOUNCED, RELEASING, FINISHED, CANCELLED, HIATUS |
| releaseYear | Integer from 1900 through 9999 |
| season | WINTER, SPRING, SUMMER, FALL (reported release quarter) |
| genre | One exact selected-source genre label |
| theme | One exact selected-source theme label |
| demographic | One exact selected-source demographic label |
| studio | One exact selected-source studio label |

All supplied filters combine with AND before title/ID sorting and pagination. Input labels are trimmed (stored ASCII-space padding is ignored) and matched case-insensitively as whole values, without wildcard, regular-expression or fuzzy matching. Labels are at most 256 UTF-8 bytes; q is trimmed and at most 2,048 bytes. NUL and invalid UTF-8 are rejected. q continues to search canonical/native titles and imported synonyms by literal case-insensitive substring. Values are SQL parameters, never query fragments.

Omit a filter to disable it. Empty, repeated or invalid recognized filter parameters return 400 invalid_catalog_filter; malformed query encoding is rejected. Existing limit/offset clamping remains (1–100 results, offset 0–100,000). Unknown query keys keep their existing ignored behavior. Enums match their uppercase spelling.

Missing genre/theme/demographic/studio/year/quarter metadata cannot satisfy that filter. Legacy tags do not masquerade as themes or demographics. Filters select reported metadata, not verified accuracy, complete catalog coverage or confirmed availability. The endpoint remains public and does not use account progress/follows.

PostgreSQL filters use a read-only transaction, two-second statement timeout and three-second overall context deadline. No total-count aggregation, arbitrary sorting, taxonomy inference, source activation, schema migration or grant expansion is added. Reads still use the existing no-store/security header boundary. Local mode filters only known starter scalar fields; source-label filters return an empty list.

See [provider approval](PROVIDERS.md), [catalog feeds](CATALOG_FEEDS.md) and [current implementation](PROJECT_CONTEXT.md).


## Dashboard

Open Catalog filters in Add anime, select scalar fields or enter source labels, then Apply catalog filters. Draft edits do not fetch until applied. Filters work without a title query and combine with title/alias search when supplied. Clear catalog filters removes all applied criteria; hiding the controls preserves them. Inputs remain editable during a request.

Filtered searches do not display unfiltered starter fallbacks while pending or failed. A local retry action repeats the current search, and response revisions discard older filter/query results. Account changes and successful tracker mutations reset applied filters alongside the existing query reset. Values and results are never persisted in browser storage.

Eight matching suggestions appear initially; Show more catalog matches expands the returned results eight at a time, up to the current 100-result request. At that cap the dashboard explicitly asks for a narrower search. Broader catalog pagination/facets remain future work; there is no inferred taxonomy or hidden spoiler-bearing suggestion list.


## Year ranges, episode ranges and ordering

Use minYear/maxYear (1900–9999) and minEpisodes/maxEpisodes (0–100000) for inclusive optional bounds. For example:

GET /api/v1/anime?minYear=1990&maxYear=2005&minEpisodes=12&maxEpisodes=26&sort=NEWEST

Unknown years/counts do not satisfy active ranges. Known episode count zero is distinct from unknown. Reversed ranges, empty/repeated bounds and non-integer values return 400. Exact releaseYear remains an additional AND filter.

sort accepts TITLE (default), NEWEST or OLDEST. Year ordering puts unknown years last and breaks ties by canonical title/ID before pagination. Keep all filters and sort when following offset hints; ingestion can still shift offset pages.

The dashboard provides Earliest/Latest release year, Minimum/Maximum episodes and Catalog sort order in its explicit Apply form. Clearing filters clears those values too.

## Cursor pagination

Start with GET /api/v1/anime?cursor=start&limit=24 and any supported filters or sort. When nextCursor is present, pass it as cursor with the same query, filters and sort. The page size can change. An absent nextCursor means the current page has no successor. Cursor requests reject offset, malformed or repeated cursors, and cursors issued for different filters. Existing offset requests keep their existing response fields.

Each page starts after the previous page's year/title/ID or title/ID key. Insertions and removals before the key do not shift later pages. Changes to titles, years or filter membership can still affect traversal because the catalog is live across requests.

Use `confirmedRelease=true` to find titles with a confirmed release on any track. The filter reads the latest fact for each episode and track and requires a recorded release time at or before the current database time. Omit the parameter or use `false` to include titles regardless of release evidence.

Use `metadataProvider=<provider-key>` to require the exact primary source of a title's retained metadata. This combines with the other filters and is bound to the catalog cursor. The dashboard can suggest provider keys from `GET /api/v1/anime/facets?field=providers`. Suggestions use retained catalog attribution, including older metadata from providers whose network access is disabled.
