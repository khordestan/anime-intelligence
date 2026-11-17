# Reviewed unused-identity retraction

This workflow retires an incorrect exact provider link that has not contributed facts. It does not delete or merge a canonical title, move private tracking data, relink a retired source ID to a different title, retract confirmed releases, erase audit history or resolve licensing erasure requests.

## Review and preview

Apply migration 000022 and refresh catalog-writer and catalog-mapper grants before using the matching binaries. Provision deployments/catalog-retractor-role.sql and its password out of band. Use only CATALOG_RETRACTION_DATABASE_URL, with no fallback to runtime, mapping or ingestion credentials.

Obtain the expected row UUID using the dedicated role and an exact reviewed public identity:

```sql
SELECT id, anime_id FROM anime_external_ids
WHERE provider = 'synthetic-secondary' AND external_id = 'work-1';
```

The mapping UUID is a compare-and-set token for that exact link instance. It is not a user/account ID. Review the source match independently, then supply version-1 JSON:

```json
{
  "version": 1,
  "provider": "synthetic-secondary",
  "reviewedAt": "2026-11-17T00:00:00Z",
  "reference": "https://example.test/identity-correction",
  "retractions": [{
    "externalId": "work-1",
    "animeId": "10000000-0000-4000-8000-000000000001",
    "expectedMappingId": "20000000-0000-4000-8000-000000000001"
  }]
}
```

Input is capped at 1 MiB and 1000 distinct provider identities/canonical targets. UUIDs normalize to lowercase and times to UTC microseconds. Source identifiers, public HTTPS references, duplicate checks, actual review time and strict JSON rules match catalog-map. Retraction remains possible after a source is disabled; enabling it is not required.

```sh
go run ./cmd/catalog-retract -input=/path/to/reviewed-retractions.json
go run ./cmd/catalog-retract -input=/path/to/reviewed-retractions.json -apply
```

Preview performs the same writes/checks and rolls back. Only -apply commits. Output contains applied/retracted/unchanged/auditRecords counts; fixed failures do not expose IDs, references, source content or connection values. Connections/cleanup and SQL are bounded. All rows share the catalog transaction advisory lock. A late failure rolls back earlier link removals and tombstones.

## Dependencies and permanent retirement

The complete batch is refused if a link has primary/resolved metadata authority, selected field data or retained field-review evidence (including restored choices), schedule observations, any retained episode revisions, source-owned relation edges, queued source jobs, non-mapping source archives, or retained primary relation claims targeting its external ID. These dependencies require a separate reviewed correction strategy. Dormant queue rows count as dependencies too; this command does not prune them or change worker configuration.

Successful retirement removes only the exact mapping row and inserts immutable review evidence into catalog_identity_retractions: provider, external ID, canonical/mapping UUIDs, review time, public reference and normalized decision hash. Canonical titles, metadata, private follows/progress/ratings and schedules remain intact. Public exact lookup stops resolving that provider ID and details omit the removed link. Existing mapping evidence is retained.

An identical current retirement review replays without writes; a different reference/time/expected UUID conflicts. A database trigger serializes identity insertion/update with the same catalog lock and rejects retired provider/external-ID pairs on any canonical target. This also fences older mapper/import binaries and rolls back an import's attempted new title/watermark. Re-enabling the source does not restore an identity. New approved source IDs can still be mapped normally.

The dedicated role reads public dependencies, inserts tombstones and deletes identity rows. It cannot alter canonical facts, tombstones, source approval, schedules, audit archives or private data. The role remains a trusted public-catalog operator: direct SQL outside this tool must obey the same locking/review contract; provisioning does not remove pre-existing excess grants. API roles gain no tombstone/reference access. The common writer guard recognizes only this explicitly reviewed new public table.

Downgrade refuses while tombstones remain, preserving replay and anti-resurrection guarantees. No automatic restore or reassignment exists. Canonical entity merges, dependent-data corrections, schedule/event retractions, archive erasure policy and queue retirement remain unfinished. No provider, source, production host or deployment is activated.
