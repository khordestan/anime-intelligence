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
