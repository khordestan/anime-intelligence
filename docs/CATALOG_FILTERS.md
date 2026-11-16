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
