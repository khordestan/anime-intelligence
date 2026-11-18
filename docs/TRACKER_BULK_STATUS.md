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
