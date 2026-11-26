# Library browsing

Use **Search your library** for a trimmed, case-insensitive title substring within your loaded library. This is separate from public catalog search and does not search private notes or imported aliases.

Watch status and **Library rating filter** combine with the title query. Ratings can be any, rated, unrated, or an exact score from 1 to 5. Unrated entries never match a numeric score.

**Library sort order** supports the server's default order, title A–Z, highest/lowest rating, or most episodes watched. Both rating orders place unrated titles last. Ties use title and entry ID. Watched-episode order uses the current pass's recorded count, including counts from exact selections; it does not estimate completion percentage or lifetime viewing time.

**Clear library filters** restores every library control, including watch status and ordering. A matching count and empty-state message explain hidden entries. Header statistics continue to describe the whole loaded library.

These controls are in-memory presentation state, reset on account changes and reload, and never write tracker records or browser storage. They operate on the current tracker response; they do not imply server-side pagination or full-catalog search.

Validation: Playwright covers combined filters, rating/tie ordering, unrated behavior, reset/default-order preservation and no writes. The existing account-switch regression also checks that all three new controls reset. Hosted CI is required; no local executor is available.

**Library progress mode** filters the current pass to exact episode selections or episode count, combined with all other filters. Missing `exactProgress` in count-mode/local entries is treated as count mode. The filter does not infer gaps, completion, or availability, and refresh preserves it. Clear, account changes, and reload reset it.

Open **Library summary** to see status totals, exact progress usage and separate series and season rating distributions. Episode estimates use the remaining counts from Watching and On Hold entries with a positive recorded total. Entries beyond a corrected total contribute zero remaining episodes. The summary reports how many active entries are represented. The dashboard calculates these values from the current library, including entries hidden by active library filters. Archived viewing passes are outside this calculation.
