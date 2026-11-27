# Catalog snapshot boundary

The provider-neutral feed package and cmd/catalog-check validate offline metadata and preview confirmed episode-release events. They do not write catalog tables, alter tracker progress, send alerts, or enable a network provider.

## Contract

Version 1 snapshots contain a provider key, observation timestamp and partial list of titles. Identities are the pair (provider, externalId); titles are never matched by name. Metadata includes native names, aliases, synopsis, media/status, year/season, nullable episode total, genres, themes, demographics, tags, studios, typed season/title relations and per-track episode schedules/confirmed release timestamps. All fields originate from the named snapshot provider and observation time. This is snapshot-level provenance; field-level canonical selection remains future work.

Null episode totals mean unknown. Relations may target records outside the partial snapshot. A franchise season is a separately identified title linked by PREQUEL/SEQUEL. The `season` field holds the airing quarter. A reviewed primary snapshot may supply a positive `seasonNumber` and an optional positive `partNumber` for a split season. Zero or omission leaves the number unknown, and a part requires a season number. Catalog details show these source labels. They do not change canonical IDs, watch progress, ratings or follow scope. Genres/themes/demographics/tags do not automatically become curated taxonomy observations. Themes and demographics retain their provider categories; legacy merged tags may also remain for compatibility. Missing categories stay empty instead of being inferred from other labels. Synopses/tags may contain spoilers and are not automatically published.

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

## Measure field presence before importing

Run `go run ./cmd/catalog-check -input snapshot.json -coverage` to append aggregate field and per-track row counts to the validation report. Counts cover the validated snapshot and include known zero episode totals. Unknown format, status and release year and missing text or lists contribute zero to their respective fields. Scheduled and confirmed release counts are independent, so a row containing both timestamps contributes to both counts. Reusing the report on different candidate feeds can show which fields each sample covers. It measures recorded presence within the supplied set. Field accuracy, coverage of the wider catalog and licensing require separate review.

## Preview schedule moves

Run `go run ./cmd/catalog-check -previous previous.json -input next.json -schedule-changes` to preview stated schedule moves alongside confirmed release events. Moves compare matching external title IDs, episode numbers and tracks from two ordered snapshots of the same source. Each item includes the before and after times and whether the stated schedule moved earlier or later. New, omitted and unchanged times and rows with a release confirmation contribute no schedule move. The same comparison uses PostgreSQL timestamp precision. A first snapshot produces an empty preview. This offline comparison creates no alerts and keeps schedule moves separate from release confirmations.

## Check reviewed field quality offline

Use a policy file to check minimum title counts, field presence in the candidate snapshot and agreement with a separate reviewed reference. Run:

```sh
go run ./cmd/catalog-check -input snapshot.json -reference reviewed.json -quality-policy policy.json
```

The example policy below is for a synthetic test. Choose field thresholds and minimum sample sizes after review for each actual source and scope.

```json
{
  "version": 1,
  "minTitles": 2,
  "presence": {"seasonNumber": 0.5},
  "accuracy": {"episodeCount": {"minJudgments": 2, "minCorrectRate": 0.9}}
}
```

Staff and character credits can be checked independently for presence or reviewed accuracy. Credit agreement compares complete name and role pairs regardless of order or case. The reports emit aggregate counts.

Schedule timestamps are judged separately by exact provider title, episode number and track. Use keys such as `scheduledAt.SUB` and `releasedAt.DUB` in an accuracy policy. The track suffix may be `RAW`, `SUB`, `DUB` or `UNKNOWN`. A known reference timestamp contributes one judgment. Equality compares the exact instant even when offsets differ. A missing row or candidate timestamp contributes a missing judgment. Extra candidate rows contribute no judgments. These observations describe predictions and confirmations under the recorded source, with no claim about streaming rights or availability elsewhere.

Presence uses all candidate titles as its denominator. Accuracy counts correct, missing and mismatching values against exact reviewed source IDs. Unknown reference values produce no judgment. If fewer values are judged than the policy requires, that field fails even when every available judgment matches. The report includes each numerator, denominator, rate and decision. A valid report that misses a threshold is printed before the command exits with status 1. A presence-only policy needs no reference. These checks use files only and neither certify production catalog coverage nor activate an operational SLO.

## Reviewed staff and character credits

An approved offline primary snapshot can include `staffCredits` and `characterCredits`. Each list accepts up to 128 distinct name and role pairs. The importer keeps the supplied labels for the canonical title under its primary source attribution. The labels do not establish people or character identities across titles. A subsequent accepted primary observation replaces the prior lists for that title.

Public title details include staff credits. To avoid revealing character appearances by default, title details return only whether character credits are available. The dashboard requests those names and roles from a separate endpoint when the viewer chooses **Show character credits**. Displayed credits use the selected primary source and its observation timestamp. The offline import requires the existing source approval and no network adapter is enabled by these fields.
