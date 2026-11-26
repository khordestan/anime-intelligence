# Inactive catalog freshness alert rules

deployments/monitoring/catalog-alerts.yml is an optional example, tested with promtool in CI. It is not included in active hosting/collector configuration and no receiver, target or service is activated.

Only targets explicitly labeled anime_catalog_monitor="true" are in scope. A recording rule accepts snapshots from reachable targets that are no older than 180 seconds and no more than 60 seconds ahead of monitoring time. Match keys are job and instance; preserve target labels through the collector.

Warnings distinguish unreachable targets, missing snapshots, stale snapshots and clocks ahead (two-minute persistence). Title staleness requires a healthy snapshot and five minutes of persistent nonzero stale-title counts; future title timestamps require healthy data and two minutes. Title age uses the operator's catalog-status -stale-after threshold. These are example operational warnings, not a completeness/accuracy SLO or release-availability guarantee.

Tune snapshot freshness to collection cadence before operational use. The inactive catalog collector examples publish the snapshot atomically. Approved target inventory and receiver routing require separate configuration. Down/missing/stale/future monitoring data suppress title alerts, avoiding claims based on old metrics. Missing per-title gauges do not imply healthy coverage. Field absence, source approval, fetch-job failures and background release delivery require separate policies.

Synthetic tests cover persistence/recovery, stale/future suppression, missing targets despite another healthy target, down versus missing, future title observations and unmarked-host exclusion. No database/account/provider payloads appear in alert labels or summaries.
