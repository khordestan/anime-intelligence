# Reviewed label-field choices

An explicit operator review can select genres, studios, synonyms, themes, demographics or tags from an approved, exactly mapped secondary source. It replaces only the named field's label array, including an explicit empty array. Canonical names, synopsis, status, totals, episodes, relations and recommendation evidence are outside this workflow.

The latest primary observation is retained separately in catalog_base_metadata; catalog_metadata is the resolved projection used by details, alias search, filters and label discovery. New primary imports update their base facts and reapply active choices in the same transaction. Restoring BASE uses the newest retained primary value, not the value seen when a choice was first made. Opt-in source metadata archives still retain the primary observation, not a mixed-source projection.

## Manifest and review

Use version 1 JSON with a public HTTPS review reference, actual source observation time, review time and expected current revision. A field with no prior decision has revision zero. Existing revision/mode/source information is available through public catalog details fieldSources; review references, decision hashes, raw base payloads and audit payloads have no API grants.

```json
{
  "version": 1,
  "reviewedAt": "2026-11-17T00:00:00Z",
  "reference": "https://example.test/metadata-review",
  "selections": [{
    "animeId": "10000000-0000-4000-8000-000000000001",
    "field": "genres",
    "mode": "SELECT",
    "expectedRevision": 0,
    "provider": "synthetic-secondary",
    "externalId": "reviewed-one",
    "values": ["Adventure"],
    "observedAt": "2026-11-16T00:00:00Z"
  }]
}
```

The manifest is capped at 1 MiB and 100 distinct title/field decisions. Arrays allow at most 256 trimmed, nonempty, case-insensitively unique UTF-8 labels of at most 256 bytes. Provider identity must already map to that canonical title and its source must be independently approved. Rights/scope/accuracy and historical retention for the explicit audit remain the operator's review responsibility; a reference URL is not automated licensing approval. No inference from tags to genres, demographics or taste scores occurs.

To restore a field, use mode BASE with expectedRevision and omit provider, externalId, values and observedAt. Revisions remain retained after restoration. An exact replay of the current decision is idempotent; stale/conflicting revisions and non-increasing review times fail the entire batch. Replaying an older selection after restoration cannot resurrect it.

## Run and credentials

Apply migration **000019**, refresh catalog-writer/reader and API runtime grants, and provision deployments/catalog-field-curator-role.sql with its password out of band. Use a dedicated CATALOG_FIELD_DATABASE_URL; ingestion/runtime credentials are not fallbacks.

```sh
go run ./cmd/catalog-fields -input=/path/to/reviewed-fields.json
go run ./cmd/catalog-fields -input=/path/to/reviewed-fields.json -apply
```

The default runs all writes/constraints and rolls back. Only -apply commits; output contains counts, never field values, references or credentials. All choices, audit records and resolved payload changes are atomic under the shared catalog advisory lock. A late mapping/revision/audit failure rolls back earlier choices and projected values. SQL statements and connection/cleanup time are bounded.

The curator cannot create titles, modify canonical columns, alter primary observations, import schedules, enable sources, change source policy URLs, read private account tables or delete audit/decision rows. The factual worker can read/apply choices but cannot curate them. API roles have only the narrow public provenance columns on operator tables.

A transaction-local projection marker prevents older ingestion binaries from overwriting resolved choices or leaving primary observations stale. This is a compatibility protocol, not a reduced-privilege security boundary; owners can perform explicit repairs. Use matching binaries/schema/grants on upgrade. Readiness fails when the required provenance schema/grants are absent. Downgrade refuses while decision or audit history remains.

## Provenance, coverage and limits

Details retain the primary provider and expose per-field provider, observation time, mode and revision. The dashboard shows source/time without exposing themes/tags before spoiler reveal. Source-presence diagnostics use retained primary observations, so selected secondary labels are not credited to the primary provider. Those aggregate coverage measures describe primary observations, not resolved catalog completeness.

The latest base row is not a raw provider response or historical archive. Audit rows retain the explicitly reviewed field decisions; pruning/licensing corrections require an operator policy. Source disabling stops new SELECT decisions but does not automatically remove previously reviewed data or force BASE. Source corrections/retractions, numeric/title/prose field arbitration, entity merges, comprehensive approved feeds and confirmed availability remain unfinished. No source activation, real metadata corpus or deployment was performed.
