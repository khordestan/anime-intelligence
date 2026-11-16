# Catalog snapshot boundary

The provider-neutral feed package and cmd/catalog-check validate offline metadata and preview confirmed episode-release events. They do not write catalog tables, alter tracker progress, send alerts, or enable a network provider.

## Contract

Version 1 snapshots contain a provider key, observation timestamp and partial list of titles. Identities are the pair (provider, externalId); titles are never matched by name. Metadata includes native names, aliases, synopsis, media/status, year/season, nullable episode total, genres, themes, demographics, tags, studios, typed season/title relations and per-track episode schedules/confirmed release timestamps. All fields originate from the named snapshot provider and observation time. This is snapshot-level provenance; field-level canonical selection remains future work.

Null episode totals mean unknown. Relations may target records outside the partial snapshot. A season is a separately identified title linked by PREQUEL/SEQUEL; WINTER describes an airing quarter, not a franchise season number. Genres/themes/demographics/tags do not automatically become curated taxonomy observations. Themes and demographics retain their provider categories; legacy merged tags may also remain for compatibility. Missing categories stay empty instead of being inferred from other labels. Synopses/tags may contain spoilers and are not automatically published.

Input is capped at 8 MiB. Unknown fields, trailing documents, invalid identities/enums, duplicates, self relations, cyclic PREQUEL/SEQUEL chronology, invalid episode bounds and future observations/releases fail closed. Errors do not print raw input. This validates structure, not licensing, truth, source trust or completeness.

## Local preview

Run:
- go run ./cmd/catalog-check -input=/path/to/current.json
- go run ./cmd/catalog-check -input=/path/to/current.json -previous=/path/to/previous.json

The preview compares only the two supplied snapshots: confirmations at or before the previous observation are historical, not new alerts, and times use PostgreSQL microsecond precision. It has no retained title history, identity mappings or durable event ledger. The persistent importer can differ after intervening partial feeds because it retains per-title observations and canonical event identities. Use importer validation/dry-run boundaries when reviewing actual database changes.

Synthetic example:

```json
{"version":1,"provider":"synthetic","observedAt":"2026-01-02T00:00:00Z","titles":[{"externalId":"sky-s1","title":"Synthetic Sky","mediaType":"TV","status":"RELEASING","episodeCount":null,"genres":["Adventure"],"relations":[{"externalId":"sky-s2","type":"SEQUEL"}],"episodes":[{"number":1,"track":"SUB","scheduledAt":"2026-01-01T12:00:00Z","releasedAt":"2026-01-01T12:05:00Z"}]}]}
```

Without a previous snapshot, establish a baseline and emit no historical alerts. Later snapshots must advance observation time and use the same provider. An elapsed schedule never creates an event. Confirmed releases create deterministic SHA-256 keys scoped to provider, title, episode and track; timestamp corrections retain the key. Output ordering is stable.

## Required persistence and provider work

No external provider is approved or enabled. Before a network adapter: document current terms, attribution, redistribution/cache limits, allowed fields, request limits and deletion obligations. Public API access does not establish redistribution rights.

Before applying snapshots: resolve provider IDs to canonical IDs, store permitted evidence/field provenance transactionally, reject stale observations against a durable high-water mark, retain schedule revisions and validate continuity against the entire persisted graph (the decoder only checks the submitted snapshot). Never delete canonical titles based on absence from a partial feed. Provider identities remain distinct until reviewed resolution merges them.

Before delivering alerts: retain a durable unique event-key ledger across partial snapshots/corrections, map events to canonical identities for cross-provider deduplication, scope follows/preferences to authenticated owners, and enqueue notifications atomically. Pure comparison cannot guarantee durable or cross-provider deduplication. New-season announcements, retractions, provider conflicts, region/language availability, user episode tracking and in-app delivery remain pending.

## Persistent administrator import

Migration 000011 adds catalog_sync_state (per-source high-water marks) and catalog_metadata (latest structured title snapshot plus source/time). Apply migrations before using:

```sh
go run ./cmd/catalog-import -input=/path/to/reviewed.json
```

Supply CATALOG_DATABASE_URL through the environment with dedicated catalog administrator credentials. DATABASE_URL is deliberately not a fallback. The command has a two-minute deadline, prints aggregate counts only and never prints raw provider/connection errors. It never starts an HTTP listener or sends notifications.

An administrator must review terms, permitted fields, storage/redistribution and attribution first, then register the source in data_sources with a policy_url and enabled=true. This is an explicit operational gate, not automated legal approval. No external sources are seeded/enabled. Latest structured metadata is retained by default. Optional -archive stores reviewed provider-neutral observations only after historical-retention rights are reviewed; arbitrary raw responses are never archived. -preview validates the whole database import and rolls back. See [metadata history](CATALOG_HISTORY.md).

All importer writers use a global transaction advisory lock. Imports atomically resolve exact provider IDs, create canonical titles, update authoritative core metadata, preserve latest structured provenance, retain schedule revisions, resolve relations and advance the observation high-water mark. Equal/older observations fail; PostgreSQL microsecond timestamp precision is used. No title-name matching or cross-provider merging occurs. Use [reviewed identity links](CATALOG_IDENTITIES.md) to pre-map exact provider IDs without ad hoc SQL. Pre-mapped titles can acquire an initial source authority; a different source cannot overwrite an existing authority. Conflicting relation/schedule authority and continuity cycles roll back the entire import, including new IDs and the high-water mark.

Partial feeds never delete titles, confirmed releases or previously resolved relations. Retained source-title relations are revisited when their targets arrive later. The whole persisted prequel/sequel graph is checked for cycles. Public catalog details expose direct relations with linked canonical titles/media/status and derive only the inverse PREQUEL/SEQUEL direction for navigation. Reciprocal assertions deduplicate. Other directed types do not acquire inferred inverses or imply viewing order. This read-time view does not mutate provider provenance or fabricate a franchise season number. Canonical episode release events use unique provider-independent keys in domain_events. First provider imports establish release history. On later batches a new title may emit its first confirmed release when newer than the previous provider observation; historical new titles still baseline. Known titles compare their own prior observation, so unrelated partial updates do not hide releases. Older confirmations establish history only. Timestamp corrections append schedule revisions without duplicating an event. Persistent event keys survive intervening partial feeds.

Runtime API credentials receive no new catalog write grants. This command currently requires administrator-managed source registration and credentials. A separate restricted factual writer, opt-in bounded Wikidata worker, public freshness/attempt diagnostics and owned polling notifications are implemented. Comprehensive approved network sources, durable per-title scheduling, field arbitration, retractions, archive retention policies and background delivery remain pending.

## Explicit CC0 factual fetch

catalog-fetch retrieves a bounded set of supported Wikidata anime-work items as a neutral feed. It has no database access and activates no scheduled source. See [provider capabilities](PROVIDERS.md) for mapping, CC0 scope and request etiquette. The factual output intentionally leaves release status/availability and unsupported fields unknown; fetch observation time does not prove every upstream statement is current.
