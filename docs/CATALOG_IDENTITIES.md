# Reviewed canonical identity links

catalog-map replaces ad hoc SQL for adding an exact provider identity to an existing canonical anime title. It does not compare names, create/merge/delete titles, remap existing identities or transfer metadata authority.

Use a reviewed version-1 JSON file:

```json
{"version":1,"provider":"synthetic","reviewedAt":"2026-11-16T00:00:00Z","reference":"https://example.test/identity-review","mappings":[{"externalId":"work-1","animeId":"10000000-0000-4000-8000-000000000001"}]}
```

The file is capped at 1 MiB and 1000 links. Provider/external IDs use the same bounded identifier character set as feeds. Canonical UUIDs must already exist. One source ID per provider/title and one canonical title per provider/source ID are enforced. Duplicate mappings, unknown fields, future review times and credential/query/fragment-bearing or non-HTTPS review references are rejected. Reference syntax is not factual or licensing approval; review the match and source rights independently.

## Run

Provision deployments/catalog-mapper-role.sql and its password out of band. Use a separate CATALOG_MAPPING_DATABASE_URL; DATABASE_URL/CATALOG_DATABASE_URL are not fallbacks. The CLI rejects elevated/owner/member roles or any private-table/column privileges, including watch-cycle history. Existing roles require a privilege review; provisioning does not revoke pre-existing extras.

```sh
go run ./cmd/catalog-map -input=/path/to/reviewed-identities.json
go run ./cmd/catalog-map -input=/path/to/reviewed-identities.json -apply
```

The default performs transactional writes/constraint checks and then rolls everything back. linked/unchanged/auditRecords are projected counts and applied=false. Only -apply commits. The one-minute command prints aggregates and fixed failures, never connection values, raw source content or supplied IDs.

The source must already be administrator-enabled with a policy URL. All links and audit records share the catalog transaction advisory lock with import/curation. A late conflict/missing target rolls back the full batch. Identity changes and approved-source edits outside these tools must honor that lock too.

Applied reviews append bounded IDENTITY_MAP evidence (record_type IDENTITY_MAP) to source_records: provider, exact IDs, microsecond review time and reference. Normalized per-link hashes suppress identical replays; a new reviewed time/reference retains another review without changing identity. No reviewer names, account IDs, private notes or arbitrary provider bodies are accepted.

The restricted mapper role can select public identity/source metadata and append identity/audit rows. Narrow external_url/display_name UPDATE grants support PostgreSQL row locks; it cannot update the canonical/provider/ID columns, enable a source, read audit payloads, modify anime metadata, delete rows or access private resources.

## Authority boundary

A first factual import can enrich a pre-linked canonical title that has no source authority. Linking a second provider to an already authoritative title does not authorize a takeover: the importer still rejects that provider's metadata/schedules. Field arbitration, correction/retraction and merging duplicate canonical entities remain separate reviewed work. This command performs no network requests, private mutations, source activation or deployment. Existing schema suffices; mapping itself changes no API write grants.

## Public read boundary

GET /api/v1/anime/lookup?provider=synthetic&externalId=work-1 resolves an exact case-sensitive identity to its canonical public title. Both query fields are required bounded identifiers; missing/unknown identities return 404 after validation. No account lookup or title-name inference occurs. JSON demo mode has no invented provider links and returns 404.

Catalog details include a provider/externalId array in the same public snapshot as metadata/relations/schedules. Source still names the authoritative metadata provider; the identity list does not transfer that authority. The dashboard renders IDs as text, with no unvalidated external links. Audit payloads, review references and external_url are not returned.

Reapply deployments/runtime-role.sql for SELECT(anime_id,provider,external_id); readiness checks the narrow read grant. No new migration is needed. Relinking/retraction and canonical entity merges remain unsupported. Mapping does not resolve retained relation claims immediately; the next import revalidates resolution and graph continuity.
