# Aggregate catalog metrics

Run cmd/catalog-status with -format=prometheus and the existing restricted catalog diagnostics connection. JSON remains the default; no HTTP endpoint, collector file, service, source or monitoring receiver is activated.

The fixed anime_catalog_ gauges report snapshot time, configured staleness threshold, canonical/observed/unobserved/stale/future title counts, stale enabled-source count, latest per-track schedule/confirmation counts and nine selected-metadata presence counts. There are no dynamic labels, provider IDs/URLs, metadata values or user records. Known zero episode totals count as present.

Presence is not accuracy, completeness or licensed availability. Confirmed episode counts are per canonical title/episode/track, not unique across tracks. Source freshness and per-title freshness remain independent. The snapshot timestamp lets operators detect stale collector output; process success alone does not keep old output fresh. Operational collection, atomic file replacement and alert routing require separate setup.

The renderer rejects invalid bounds/negative aggregates with the existing fixed diagnostic error and emits no partial text. Unit tests cover the fixed gauge contract and privacy boundary; monitoring CI validates synthetic real-renderer output with promtool. Local toolchains unavailable; exact follow-up CI pending.

## Offline field accuracy

Run `catalog-check -input=candidate.json -reference=reviewed.json` to compare a neutral candidate snapshot against an independently reviewed set of expected values. Both use the same provider key and exact external title IDs. Use comparable observation dates when judging status and other time-sensitive fields. The report records both timestamps and the number of reviewed IDs found.

The `accuracy.fields` object counts correct, missing and conflicting values for each reviewed field. Omit unknown reference values to leave them unjudged. A missing candidate title contributes missing values for its judged fields. Known zero episode totals are evaluated. Genres, studios and themes compare as normalized whole-label sets. Supply `-coverage` separately to see presence across the entire candidate. Use a representative, licensed sample before setting field targets.
