# Optional selected-metadata observation history

catalog-import retains only the latest reviewed neutral title by default. Historical retention is separately opt-in:

```sh
go run ./cmd/catalog-import -input=/path/to/reviewed.json -preview
go run ./cmd/catalog-import -input=/path/to/reviewed.json -preview -archive
go run ./cmd/catalog-import -input=/path/to/reviewed.json -archive
```

Existing invocations without -preview still commit; preview is an explicit option. A preview performs the complete database operation, including authority checks, persisted graph validation, schedule revisions, events and optional archives, then rolls back. Reported counts are projected and preview=true. It does not deliver notifications or persist a high-water mark, identities or metadata.

Use -archive only after independently reviewing the source's historical storage/redistribution and retention obligations. Source enabling and a policy URL are operational checks, not permission for indefinite retention. Configure a lawful retention/deletion process before operational use; automated pruning is not enabled.

Archives store the selected provider-neutral title (including approved prose when present), canonical ID, provider and microsecond observation time in source_records with record_type CATALOG_METADATA. They do not store arbitrary raw responses, HTTP headers, source credentials, user profiles or the unrecognized provider envelope. Each retained observation has a type-scoped hash, so repeated values at a newer observation are separate chronological observations. The existing stale-source high-water rule rejects repeated/older imports.

Archive insertion shares the catalog transaction/advisory lock. Failure rolls back metadata, identity creation, schedules, events and source state together. API runtime credentials receive no archive reads/writes. The factual worker continues to use non-archiving Import; no service/source/retention job is activated.

This is not raw provider-response archival, complete field provenance or a claim that the underlying facts are correct/current. Earlier imports made without -archive cannot be reconstructed. All fields in one retained title still originate from its selected source observation; cross-provider arbitration remains separate.

## Operator field-change summary

Apply the optional deployments/catalog-history-role.sql using migration credentials and set its password outside source control. Supply that separate reader connection through CATALOG_HISTORY_DATABASE_URL:

```sh
go run ./cmd/catalog-history -provider=reviewed-provider -external-id=exact-title-id -limit=20
go run ./cmd/catalog-history -provider=reviewed-provider -external-id=exact-title-id -limit=20 -before=2026-01-02T03:04:05Z
```

The command emits only observation times, sorted changed field names, baseline/truncated flags and an optional nextBefore cursor. It never emits canonical/provider IDs, titles, synopsis values, raw bodies or credentials. Identifiers match exactly; missing IDs return an empty entries array. It compares selected observations, not effective canonical/schedule state (a missing release confirmation can remain effective in the schedule). Array order counts as a literal change. Known episode count zero differs from an absent count.

Pages contain 1–100 newest observations, optionally strictly older than the before cursor. Use nextBefore for the next page. An extra older row supplies the comparison at a page boundary; baseline means no earlier retained observation, not an initial catalog import. Pruning or concurrent imports can change later reads; each call uses its own repeatable-read read-only snapshot. Imports without archives create unobservable gaps.

The dedicated reader has narrow catalog column SELECT grants, no private-table access or writes. The CLI rejects elevated/account-capable roles with the shared catalog privilege guard and uses a fixed search path. Reads have a two-second statement timeout, a ten-second overall command deadline, at most 101 fetched observations and a sixteen-MiB total payload bound. Oversized, malformed or duplicate-time observations fail with a fixed error rather than returning partial summaries. No HTTP history endpoint, role/source activation, pruning or restore process is enabled.
