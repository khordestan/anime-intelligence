# Reviewed secondary episode sources

A title's metadata source and episode source can be reviewed independently. The explicit schedules-only mode imports episode facts for existing exact provider identities while preserving canonical names, synopsis, totals, categories, status, relations and metadata provenance. It never creates or name-matches a canonical title. Missing mappings and existing episode/track authority conflicts fail the entire transaction.

This is a limited source-facet separation, not general field arbitration or automatic source priority. Existing schedules are never transferred to another source. A secondary episode above the primary metadata's known total is rejected, including a known zero. Full metadata imports also reject totals that would invalidate retained episodes from another source. Unknown totals remain unknown.

## Operator workflow

1. Review source rights and independently approve the source using the existing administrator-controlled policy/enabled gate.
2. Review the provider title's exact canonical season match and apply it through [catalog-map](CATALOG_IDENTITIES.md). Identity linking does not change metadata or existing schedule authority.
3. Preview a lawfully obtained neutral snapshot, then explicitly apply the same scope:

```sh
go run ./cmd/catalog-import -input=/path/to/reviewed.json -schedules-only -preview
go run ./cmd/catalog-import -input=/path/to/reviewed.json -schedules-only
```

The feed remains strictly validated, including ignored metadata fields. The metadata archive flag cannot be combined with schedules-only mode. Episode schedule revisions, explicit confirmations, release-event deduplication, per-title observation and source high-water mark commit together. A preview rolls all of them back. No raw or ignored metadata payload is archived.

The first schedules-only observation establishes each canonical title's baseline and suppresses historical confirmations, even when that provider has already observed another title. Later confirmations compare that title's own prior scoped observation. Unrelated partial updates cannot hide a new confirmation. Missing confirmations never retract an established release; timestamp corrections do not duplicate events. This mode does not turn predictions into confirmed availability: only explicit releasedAt facts can generate release events. The TVmaze adapter always supplies unconfirmed UNKNOWN-track schedules.

## Durable TVmaze jobs

Apply migration **000018** and reapply deployments/catalog-writer-role.sql. No new API/runtime grant is required. The existing worker configuration additionally accepts CATALOG_SYNC_SCOPE=schedules for TVmaze; empty/full preserves the prior full-import behavior. Wikidata's adapter has no schedule capability and rejects that worker configuration. Existing empty-ID configurations remain idle.

Every configured ID must already be mapped before scheduling or fetching. Jobs persist import_scope as FULL or SCHEDULES; configuration mismatch fails before HTTP, without resetting due times or retry budgets. The canonical identity captured during admission is rechecked before import, so an in-flight mapping correction cannot redirect the old response to another title.

A transaction-local app.catalog_import_scope marker fences updates to SCHEDULES jobs. Old queue binaries cannot claim or acknowledge that work as metadata imports. The marker is a compatibility protocol, not a reduced-privilege security boundary: the dedicated catalog writer retains its existing trusted public-catalog rights. It confers no account/private-table access and resets on commit/rollback.

For an existing job, stop the source workers and review pending work, mappings and retry outcomes before changing scope. Perform any administrator correction under the catalog advisory-lock contract, invalidate outstanding leases, preserve due times/provider cooldowns, and do not silently reset exhausted retry budgets. Configuration alone never promotes work to full metadata authority.

Acknowledgement remains atomic with the scoped import. Expired/replaced leases, changed scopes/mappings, revoked approval or late acknowledgement failures cannot leave partially imported episode facts or observation timestamps. Failures retain bounded retries and provider cooldowns.

## Provenance and rollback

The details API keeps provider as the metadata source and adds optional provider to each episode revision. The dashboard shows per-episode sources and retains TVmaze attribution even when the title's primary metadata came from another provider.

catalog_schedule_observations contains only public source/title IDs and observation times, without account data or raw content. API credentials cannot read it. Catalog writers cannot delete its baselines. Schema downgrade refuses while scoped jobs or observations remain, requiring explicit operational review/archive rather than discarding state.

This change enables reviewed secondary schedules on an existing catalog. Metadata field arbitration, entity merging/correction, comprehensive approved feeds and confirmed availability providers remain unfinished. No source activation, real catalog import, host process or public deployment was performed.
