# Exact episode progress

Migration 000013 extends the existing forced-RLS watch_progress table with an optional exact episode-number set. No new private table, worker privilege or user-ID input is added.

GET /api/v1/tracker/{entryId}/episodes returns mode COUNT or EXACT, count, numbers and nullable catalog total. COUNT preserves existing aggregate records and returns numbers=null. EXACT returns an ordered unique number array, including [] for an empty selection.

PUT the same route with {"numbers":[1,2,4]} explicitly replaces progress with that exact set and count=3. An empty array clears it. Missing/null numbers, duplicates, nonpositive numbers, values above 100000, more than 10000 selections and episodes above a known catalog total are rejected. Unknown totals remain usable. The body is bounded to 128 KiB and unknown fields/trailing JSON are rejected.

All operations require a verified authenticated account. Owner predicates, forced RLS and account/session/tracker lock ordering prevent foreign reads/writes and removal races. Foreign/missing entries both return 404. A database constraint additionally enforces unique valid arrays with matching counts.

Changing only watch status preserves exact selections. Explicit aggregate Add/Update switches back to COUNT and erases the exact set; it never guesses episode identities. The library exposes exactProgress so clients can avoid accidentally replacing exact tracking through a count shortcut. Private export includes the set; tracker/account deletion removes it.

Franchise seasons are separate canonical titles linked by relations, so each has its own entry and progress. Cour/season-number conventions, rewatches, specials with nonnumeric labels and per-language viewing histories remain pending. No local JSON exact-progress API is implemented.
