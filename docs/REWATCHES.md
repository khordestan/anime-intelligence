# Completed viewing passes and rewatches

PostgreSQL verified accounts can start a rewatch explicitly. A season is a separately identified canonical title; viewing passes do not invent franchise season numbers.

Apply migration 000015 and reapply deployments/runtime-role.sql before updating the API. Runtime startup/readiness now require fourteen forced-RLS private tables and the watch-cycle columns/grants. API credentials receive SELECT/INSERT only on archived cycles; deletion cascades from an owned tracker entry/account.

## Contract

- GET /api/v1/tracker/{entryId}/rewatches returns currentCycle, up to 100 completed-pass snapshots ordered by descending cycle, and nextBefore (zero means the end). Pass nextBefore as the before query parameter to read older snapshots.
- POST /api/v1/tracker/{entryId}/rewatches with {"expectedCycle":1} archives a COMPLETED current pass and starts cycle 2 in WATCHING status at zero progress, in count mode. It returns the updated library entry including watchCycle.
- The expected cycle is required. A stale/replayed request, non-completed current status or the cycle limit returns 409 rewatch_state_changed, with no reset. Missing/foreign entry IDs return the same 404. Invalid bodies/boundaries return 400. Anonymous/unverified accounts receive 401/403.
- At most 1000 passes can be started per title/owner; the first pass is cycle 1. This is a bounded operational limit, not a lifetime viewing statistic.
- Snapshots retain the user-declared completed pass's count or exact episode set (including gaps), then-current known/unknown catalog total and optional series rating. archivedAt is when the rewatch transition was recorded, not a reconstructed completion date.
- The current series rating stays in place. Rating/progress/catalog edits do not rewrite archived snapshots or add repeated inference weight. Notes and follows are retained. Merely changing watch status never starts a new pass.
- Count mode does not claim which episodes were seen. Rewatch reset is explicit; no count is converted into a guessed exact set.

The transaction rechecks account/session/verified state, locks the owned tracker row, archives the pass, resets progress and increments its cycle atomically. It follows account -> session -> tracker -> related data lock ordering. Two sessions using the same expected cycle cannot start two passes. Failures roll back the archive and reset together.

History is private, included in account export under watchCycles and removed by tracker/account deletion. No public catalog or source worker can access it. JSON demo mode stays single-pass; no rewatch routes are exposed there.
