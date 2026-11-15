# Reviewed feature observations

feature-import accepts an explicitly reviewed offline JSON snapshot and writes public CURATED/SERIES/NONE feature observations transactionally. It does not call a model, fetch evidence URLs or infer traits from provider tags. No production observations or personal taste histories are seeded.

Provision deployments/curator-role.sql as the migration administrator and set its password out of band. Then set CURATION_DATABASE_URL to that dedicated non-owner curator login and run:

```sh
go run ./cmd/feature-import -input=/path/to/reviewed-features.json
```

The API role and factual catalog worker have no observation-write grants. No HTTP curation endpoint is exposed. The command prints inserted/unchanged counts or a fixed error without input/connection details.

Version 1 snapshots require observedAt and 1–1000 records. Each record contains animeId, featureKey, value, confidence and reference. Values range 0–1; confidence ranges 0.0001–1. Both support at most four decimal places, matching PostgreSQL storage without silent rounding. The observation time must be nonfuture. Anime IDs must exist and feature keys must be active/non-spoiler-sensitive. Unknown fields, profile/user IDs, duplicate title/feature pairs and trailing JSON are rejected; inputs are capped at 1 MiB.

References must be credential/query/fragment-free HTTPS URLs. This syntax check does not prove public access, rights, factual support or absence of sensitive information in paths. The curator must verify those before import. Do not include source quotations, account details, private references, model guesses or ending spoilers. Reference-only evidence and CURATED classification attest an explicit editorial review, not automated verification.

All supported imports share the catalog advisory lock. Repeating the same title/feature/time/value/confidence/reference is a no-op. A conflicting value at the same time fails the entire batch. New observation timestamps append history; omitted records are not deleted. Older observations can be retained without replacing a newer eligible observation in ranking. Uncoordinated administrator SQL is outside the idempotency protocol.

Personal Match uses only qualifying referenced non-spoiler curated observations. A working import path does not establish broad feature coverage. Reviewed corpus creation, correction/retraction workflow, evidence auditing and spoiler-scoped annotations remain pending.

After precision validation, permitted floating-point representation noise is normalized to the database's four-decimal representation before comparison/insertion. Input records are copied rather than mutated; repeated identical snapshots retain replay identity.

The CLI rejects elevated/owner roles, inherited memberships and any private-table/column privileges using the same strict public-writer guard as the catalog worker. The curator role can SELECT public anime/taxonomy/observations and INSERT observations only. It cannot rewrite/delete observations, change taxonomy, enable sources or access accounts. Existing roles must be reviewed for extra grants; the provisioning script does not erase them. The low-level import library remains usable by trusted migration/test code, but the user-facing command requires the restricted role. SQL grants do not establish factual accuracy or replace editorial review.
