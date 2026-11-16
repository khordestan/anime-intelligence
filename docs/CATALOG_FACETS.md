# Available source labels

GET /api/v1/anime/facets?field=genres&prefix=A&limit=50 returns field, items, hasMore and limit. Allowed fields are genres, themes, demographics and studios. Default limit is 50, valid range 1–100; prefix is optional, literal case-insensitive and at most 128 UTF-8 bytes. Unknown/duplicate/invalid parameters return a fixed 400. SQL parameters never become query fragments.

Results deduplicate case variants, ignore ASCII-space padding, sort by folded label and choose a deterministic stored spelling. A lookahead distinguishes exactly-full from truncated pages. Narrow the prefix when hasMore is true; no total count or cursor is promised. This reflects selected stored metadata only, independently of catalog filters, and does not infer categories from legacy tags. Themes may contain spoilers: clients should request them only after explicit user intent. No dashboard auto-suggestion request is enabled yet.

The public endpoint reads no account data or raw archives. PostgreSQL reads have two-second statement/three-second overall deadlines; large aggregations can fail rather than guarantee constant database work. Local mode returns empty labels. No schema/grants, provider activation, or deployment changes.

Validation: exact-commit CI pending; synthetic PostgreSQL/HTTP coverage exercises deduplication, capacity/lookahead, category separation, literal prefixes, sparse fields and invalid parameters. Local toolchains remain unavailable.
