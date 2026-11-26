# Identity monitoring examples

These files are disabled templates and tests. Nothing installs units, starts collectors, reloads Prometheus, routes alerts or deploys the application automatically.

## Collector setup contract

Build `cmd/identity-status` and place the reviewed binary at the service's configured path. Create an `anime-monitor` service identity and a trusted collector directory; adapt users/groups/paths to your host. Allow the node_exporter identity to read group-owned 0640 metric files without allowing untrusted users to write that directory. Provision the restricted runtime `DATABASE_URL` through a protected `/etc/anime-intelligence/identity-metrics.env` file, never this repository. Queue decryption keys/SMTP credentials are not needed.

The `.service` example runs one compiled diagnostic snapshot with `-format=prometheus -output-file=/absolute/path.prom`. The CLI creates a temporary file in that directory, writes/syncs/closes the complete snapshot, sets 0640 permissions and renames it atomically. It replaces an existing destination symlink rather than following its target. Failure before rename preserves the old snapshot and removes staging files. Output errors use fixed messages; destination paths and argument values are not echoed. Use one writer per destination and a trusted local filesystem. Atomic replacement does not promise persistence through every filesystem/power failure; freshness monitoring remains necessary.

The timer example schedules the oneshot each minute after an initial one-minute boot delay. It is not enabled. Review/install/enable it only as part of an explicitly authorized operational rollout. Its filesystem sandbox allows writes only to the collector directory; the service has no Linux capabilities and does not run as root. Systemd reads the protected environment file before dropping privileges. Adapt the binary/database/filesystem permissions before activation.

Configure an already approved node_exporter with the textfile collector pointing at that directory. A local-only example uses `--collector.textfile.directory=/var/lib/node_exporter/textfile_collector --web.listen-address=127.0.0.1:9100`. The provided Prometheus config scrapes that loopback address and adds the explicit `anime_identity_monitor="true"` target label. It has no Alertmanager target or notification receiver. Do not expose these endpoints publicly as part of setup. Collect once per shared database or deduplicate snapshots; queue counts are not per API instance.

## Alert behavior

Only targets explicitly marked `anime_identity_monitor="true"` participate. A recording rule identifies reachable targets whose database snapshot is no more than three minutes old and no more than one minute ahead of monitoring time. Queue-state alerts require that fresh snapshot, preventing old counts from masquerading as a live backlog. Clock synchronization is required.

| Alert | Initial condition | Hold time |
| --- | --- | --- |
| `AnimeIdentityMonitorTargetDown` | Marked scrape target is unreachable | 2 minutes |
| `AnimeIdentitySnapshotMissing` | Reachable marked target has no snapshot metric, evaluated per job/instance | 2 minutes |
| `AnimeIdentitySnapshotStale` | Reachable target's snapshot age exceeds 3 minutes | 2 minutes |
| `AnimeIdentitySnapshotClockAhead` | Snapshot clock is more than 1 minute ahead | 2 minutes |
| `AnimeIdentityRequestCapacityPressure` | Active requests use at least 80% of shared capacity | 3 minutes |
| `AnimeIdentityRequestBacklogAge` | Nonempty request backlog is older than 5 minutes | 2 minutes |
| `AnimeIdentityDeliveryBacklogAge` | Nonempty mail backlog is older than 10 minutes | 5 minutes |
| `AnimeIdentityQueueCleanupBacklog` | Terminal work remains awaiting cleanup | 5 minutes |
| `AnimeIdentityRetainedExhaustion` | Retained attempt-exhausted records exist | 1 minute |

These are tested starting thresholds, not production-approved paging policy. All alerts use warning severity. Review timing against the one-minute collection schedule, thirty-second scrape/evaluation interval, request/proof lifetimes and measured workload. Retained exhaustion is historical evidence that can persist for a day, not proof a provider is still failing. Outcome values are gauges; no rule applies `rate`/`increase` to retained rows.

A removed scrape target cannot be detected from an absent `up` series alone; an external target inventory/configuration review must catch unintended removal. Unreachable targets are separate from reachable targets missing a textfile. Database-clock drift, collector permission/configuration failures and decryption-key problems require investigation; these alerts cannot prove end-recipient mail delivery. Receiver routing, inhibition/deduplication, dashboards, target inventory and incident response remain operational work.

## Verification

Run `scripts/install-promtool.sh /tmp/anime-promtool` on Linux amd64 for a checksum-pinned Prometheus 3.13.5 LTS test binary. The installer downloads/extracts test tooling only and starts no server. Then run:

```sh
/tmp/anime-promtool/promtool check config deployments/monitoring/prometheus.example.yml
/tmp/anime-promtool/promtool test rules deployments/monitoring/identity-alerts.test.yml
scripts/check-monitoring-units.sh
```

CI also renders actual metric output using the synthetic Go fixture and validates it with `promtool check metrics`. Alert tests cover healthy/empty queues, missing/disappearing per-target snapshots, down targets, stale/future-clock suppression, multi-target capacity matching, persistence thresholds, recovery and unmarked targets. Systemd verification substitutes `/usr/bin/true` for the example deployment binary while checking unit syntax; it does not test host-specific credentials, users or activation. Full-stack browser CI additionally runs the real CLI against its restricted disposable PostgreSQL role and checks the atomic file and permissions.

Upstream references: [rule testing](https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/), [node_exporter textfile collector](https://github.com/prometheus/node_exporter#textfile-collector), [Prometheus downloads](https://prometheus.io/download/). See [queue operations](../../docs/IDENTITY_QUEUE_OPERATIONS.md) for metric definitions and retained-outcome limitations.

## Release email scheduler rules

`release-email-alerts.yml` is an optional rule file for an authorized collector using `release-email-status -format=prometheus`. Add the rule file and mark the scrape target with `anime_release_email_monitor="true"` after configuring regular atomic publication of the snapshot. Collect once per shared scheduler database.

The rules distinguish unreachable targets, missing snapshots, snapshots over three minutes old and database clocks more than one minute ahead. A fresh snapshot triggers a warning when the oldest due scan stays more than ten minutes overdue for five minutes, or when due work stays at the attempt limit for two minutes. A due scan can belong to an account with no deliverable message. These initial thresholds describe scheduler state and require host-specific review before use.

Run `promtool test rules deployments/monitoring/release-email-alerts.test.yml` to exercise persistence, recovery, stale and future snapshots, and target scoping.

## API request metrics

Set `HTTP_METRICS_ADDR=127.0.0.1:9091` to start a separate, loopback-only `/metrics` listener on the API process. Leaving it unset disables collection. Scrape it from the API network namespace or through an authorized local collector. The public dashboard and ingress configuration do not route a metrics path.

Counters are per process and reset on restart. Request count uses fixed route groups and response classes. The rate-limit counter counts API-generated 429 responses. The duration histogram measures handler time through response completion. It excludes time spent at the ingress and requests rejected there. Unmatched paths and preflight requests are classified under `other`. Metrics omit path values, query strings, credentials, account identifiers and client addresses.

For cross-replica rates use `sum by (group, status_class) (rate(anime_http_requests_total[5m]))`. To compare p95 handler latency across groups, use `histogram_quantile(0.95, sum by (group, le) (rate(anime_http_request_duration_seconds_bucket[5m])))`. Use representative traffic when choosing alert thresholds.
