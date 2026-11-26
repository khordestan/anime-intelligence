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

The common catalog/curator/mapper role guard now fails closed for privileges on unexpected relations, RLS tables, other application schemas, views/materialized views/foreign relations, and executable user-schema SECURITY DEFINER routines. This metadata-only check supplements role provisioning; it does not revoke privileges. New legitimate public catalog tables must be reviewed into the supported table contract before a writer can use them.

Reviewed identities may now be used for explicit [schedules-only imports](CATALOG_SCHEDULE_AUTHORITY.md). That separate operation preserves metadata authority and rejects conflicts with existing episode/track ownership. Identity mapping itself still changes no facts or authority.

Incorrect unused links now have a separate [reviewed retraction workflow](CATALOG_RETRACTIONS.md) with exact-row preconditions and a permanent database retirement fence. Mapping itself still cannot remap or restore an existing/retired source identity. Apply migration 000022 and refresh mapper/writer read grants before upgrading. Dependent-data corrections and canonical merges remain unsupported.

## Preview canonical merge impact

Run `catalog-merge-impact -source=<canonical-id> -target=<canonical-id>` against an isolated review database with `CATALOG_MERGE_REVIEW_DATABASE_URL`. Use an authorized operator connection capable of reading a complete snapshot across forced row policies. The command rejects partial visibility and uses a bounded, read-only transaction.

The JSON report counts direct foreign-key references for both titles. It includes known indirect references through release events, notifications, pending email and browser push jobs, provider archive records and catalog fetch jobs. It also reports aggregate conflicts for overlapping provider mappings, episodes and tracks, tracker ownership, follow scopes and edges that would become self-references. It contains no owner IDs, account content or raw source records.

Counts describe the inspected snapshot. Before applying any canonical merge, separately review field authority, differing watch histories and ratings, release event handling and audit retention. The command has no apply option.

## Merge a reviewed canonical placeholder

Apply migration 000031, reapply runtime grants and provision the dedicated `deployments/catalog-merge-reviewer-role.sql` role. With its connection supplied through `CATALOG_PLACEHOLDER_MERGE_DATABASE_URL`, run `catalog-merge-placeholder -retired=<placeholder-id> -survivor=<canonical-id> -reference=<public-review>` to preview. Review the returned mapping IDs and add `-apply` with a repeated `-expected-mapping-id=<uuid>` for each returned ID.

The operation transfers only exact provider identity rows that fit unoccupied provider slots. It refuses retirement when the placeholder has tracker, rating, continuity, review, release, source archive or other dependent records. It checks references and the expected mapping IDs again during apply. It reserves the retired UUID and keeps a public redirect to the survivor. Public title, details, continuity and schedule-history reads through that UUID use the survivor. For new writes, use the canonical ID returned by those reads. An exact review replay is idempotent.

Use a maintenance window for apply because the operation briefly takes an exclusive catalog-title lock to fence new dependencies. Run the impact inventory separately for a placeholder that fails these checks. The merge reviewer only receives mapping IDs on success and a generic dependency refusal on failure.
