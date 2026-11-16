# Aggregate catalog metrics

Run cmd/catalog-status with -format=prometheus and the existing restricted catalog diagnostics connection. JSON remains the default; no HTTP endpoint, collector file, service, source or monitoring receiver is activated.

The fixed anime_catalog_ gauges report snapshot time, configured staleness threshold, canonical/observed/unobserved/stale/future title counts, stale enabled-source count, latest per-track schedule/confirmation counts and nine selected-metadata presence counts. There are no dynamic labels, provider IDs/URLs, metadata values or user records. Known zero episode totals count as present.

Presence is not accuracy, completeness or licensed availability. Confirmed episode counts are per canonical title/episode/track, not unique across tracks. Source freshness and per-title freshness remain independent. The snapshot timestamp lets operators detect stale collector output; process success alone does not keep old output fresh. Operational collection, atomic file replacement and alert routing require separate setup.

The renderer rejects invalid bounds/negative aggregates with the existing fixed diagnostic error and emits no partial text. Unit tests cover the fixed gauge contract and privacy boundary; monitoring CI validates synthetic real-renderer output with promtool. Local toolchains unavailable; exact follow-up CI pending.
