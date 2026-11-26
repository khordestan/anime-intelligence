# Bulk tracker status edits

Send a bearer-authenticated PATCH to `/api/v1/tracker/bulk-status` in PostgreSQL mode from a verified account.

```json
{
  "watchStatus": "ON_HOLD",
  "entries": [
    {"id": "10000000-0000-4000-8000-000000000001", "expectedCycle": 1}
  ]
}
```

Supply one to 25 distinct tracker entry IDs and their current `watchCycle` values. The response includes the updated entries in request order. The operation updates only watch statuses and modification timestamps, preserving episode progress, ratings, notes, and rewatch history.

The whole batch rolls back if any entry is unavailable. Missing and foreign entries return 404. A stale viewing pass returns 409, and invalid input returns 400. Refresh the library before retrying a stale request. The endpoint uses the existing verified-owner transaction and row policies.

In the dashboard, open **Edit multiple statuses** in My Library, select entries, choose a status, then apply it. If a selected entry changed viewing pass or was removed, the dashboard refreshes the library and asks you to select the entries again.


## Advance count-mode episode progress

In My Library, select up to 25 eligible entries and choose **Add one watched episode to selected**. The server advances all selected counts in one verified-owner transaction. Watch statuses and ratings stay unchanged. Exact episode selections remain available through each title's episode editor.

`PATCH /api/v1/tracker/bulk-progress` takes `entries` with `id`, `expectedCycle` and `expectedEpisodesWatched`. A retry with the old counts receives a conflict. A stale viewing pass, stale count, exact selection, or exhausted known total rolls back the entire batch.

For count-mode entries with a known episode total, **Set selected watched counts to known totals** sends each selected total as `expectedTotalEpisodes`. The whole batch fails if any current total differs from the displayed value. The update preserves watch status and ratings.

## Clear current series ratings

Choose **Clear series rating** and **Clear rating for selected**. The dashboard sends `score: null` and the selected entry IDs with their current viewing passes. All selected current series ratings are cleared in one transaction. An already unrated entry can be included. Progress, statuses, season ratings and prior viewing-pass archives are preserved.

## Remove reviewed library entries

Choose **Review selected entries for removal** to see every selected title, including titles hidden by a library filter. Confirm to submit up to 25 entry IDs with their exact modification timestamps and current viewing passes. The server removes the entries, notes, progress, ratings and archived passes in a single transaction. If any entry has since changed or disappeared, the whole operation rolls back and the library reloads for a new selection.
