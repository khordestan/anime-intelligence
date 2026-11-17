# Library browsing

Use **Search your library** for a trimmed, case-insensitive title substring within your loaded library. This is separate from public catalog search and does not search private notes or imported aliases.

Watch status and **Library rating filter** combine with the title query. Ratings can be any, rated, unrated, or an exact score from 1 to 5. Unrated entries never match a numeric score.

**Library sort order** supports the server's default order, title A–Z, highest/lowest rating, or most episodes watched. Both rating orders place unrated titles last. Ties use title and entry ID. Watched-episode order uses the current pass's recorded count, including counts from exact selections; it does not estimate completion percentage or lifetime viewing time.

**Clear library filters** restores every library control, including watch status and ordering. A matching count and empty-state message explain hidden entries. Header statistics continue to describe the whole loaded library.

These controls are in-memory presentation state, reset on account changes and reload, and never write tracker records or browser storage. They operate on the current tracker response; they do not imply server-side pagination or full-catalog search.

Validation: Playwright covers combined filters, rating/tie ordering, unrated behavior, reset/default-order preservation and no writes. The existing account-switch regression also checks that all three new controls reset. Hosted CI is required; no local executor is available.
