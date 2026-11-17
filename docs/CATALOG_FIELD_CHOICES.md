# Reviewed metadata field choices

An explicit operator review can select genres, studios, synonyms, themes, demographics or tags from an approved, exactly mapped secondary source. Label decisions replace only the named array, including an explicit empty array. Migration 000020 additionally supports typed releaseYear, season, mediaType and episodeCount choices. Canonical names, synopsis, status, episode/schedule ownership, relations and recommendation evidence remain outside this workflow.

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

The curator cannot create titles, modify canonical names/prose/status, alter primary observations, import schedules, enable sources, change source policy URLs, read private account tables or delete audit/decision rows. With migration 000020's refreshed grants it can project the four selected public scalar columns and read public schedules for total validation. The factual worker can read/apply choices but cannot curate them. API roles have only the narrow public provenance columns on operator tables.

A transaction-local projection marker prevents older ingestion binaries from overwriting resolved choices or leaving primary observations stale. This is a compatibility protocol, not a reduced-privilege security boundary; owners can perform explicit repairs. Use matching binaries/schema/grants on upgrade. Readiness fails when the required provenance schema/grants are absent. Downgrade refuses while decision or audit history remains.

## Provenance, coverage and limits

Details retain the primary provider and expose per-field provider, observation time, mode and revision. The dashboard shows source/time without exposing themes/tags before spoiler reveal. Source-presence diagnostics use retained primary observations, so selected secondary labels are not credited to the primary provider. Those aggregate coverage measures describe primary observations, not resolved catalog completeness.

The latest base row is not a raw provider response or historical archive. Audit rows retain the explicitly reviewed field decisions; pruning/licensing corrections require an operator policy. Source disabling stops new SELECT decisions but does not automatically remove previously reviewed data or force BASE. Source corrections/retractions, title/prose/status field arbitration, entity merges, comprehensive approved feeds and confirmed availability remain unfinished. No source activation, real metadata corpus or deployment was performed.

## Reviewed scalar choices (migration 000020)

For releaseYear, season, mediaType or episodeCount, omit values and supply scalar with the actual JSON type. All other identity, observation, review, CAS, audit, preview and BASE-restoration rules remain unchanged.

| Field | Accepted scalar |
| --- | --- |
| releaseYear | Integer 1900–9999, or 0 for unknown |
| season | WINTER, SPRING, SUMMER, FALL, or empty string for unknown |
| mediaType | TV, MOVIE, OVA, ONA, SPECIAL, MUSIC, UNKNOWN |
| episodeCount | Integer 0–100000, or JSON null for unknown |

```json
{"animeId":"10000000-0000-4000-8000-000000000001","field":"episodeCount","mode":"SELECT","expectedRevision":0,"provider":"synthetic-secondary","externalId":"reviewed-one","scalar":null,"observedAt":"2026-11-16T00:00:00Z"}
```

Wrap selections in the version-1 manifest above. null and zero episode totals are distinct. Numeric strings, fractional/exponent forms and mixed label/scalar values are rejected. Scalars are normalized before hashing so equivalent JSON whitespace/escaping does not defeat current-decision replay. BASE omits both values and scalar.

Resolved canonical columns and metadata payload update in the same transaction. Search/filter/sort/detail reads use the same resolved values. Label-only decisions do not rewrite canonical scalar columns. Primary refreshes retain the original scalar facts separately and keep selected values projected; restoration uses the latest base.

A known resolved episode total must contain every episode in the incoming/base observation and all retained schedule revisions, regardless of source. A smaller conflicting choice, BASE restore or primary refresh fails atomically, preserving prior facts, decisions, audits and source watermarks. Unknown totals do not assert a smaller bound. A stale choice can therefore require operator review before a title's ingestion resumes; it is never silently discarded.

Factual classification changes do not generate release or season-announcement events and do not rewrite private progress/ratings. No availability, one-episode movie total or season numbering is inferred. Downgrade refuses scalar decisions and their audit history, including restored BASE records.

Apply migration 000020 and refresh deployments/catalog-field-curator-role.sql before using scalar reviews. API grants are unchanged from 000019. Existing label manifests and replay hashes remain compatible; older projection binaries fail closed on active unknown scalar fields and must be upgraded. Scalar metadata grants do not grant canonical title, synopsis, status, source approval, episode writes or private-account access.
