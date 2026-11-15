# Identity queue diagnostics

Use `go run ./cmd/identity-status` with `DATABASE_URL` supplied out of band for the restricted runtime role. The command requires the current migrated schema (through 000010) and normal runtime grants. It rejects owner/superuser connections using the same guard as the API. It does not need SMTP settings or encryption keys, contact a mail provider, start workers, decrypt envelopes, or expose an HTTP endpoint.

The command reads both queues from a single repeatable-read, read-only transaction and writes one versioned JSON object to stdout by default. Use `-format=prometheus` for bounded Prometheus text exposition. Errors print fixed messages to stderr without connection details. Exit 0 means the snapshot was read and encoded successfully; exit 1 means missing/invalid configuration, database/query failure or output failure. Exit status is not a delivery-health verdict.

## Fields

`version` is 1. `asOf` is the database transaction time used for every count. `requestCapacity` is the shared active-request admission limit (currently 256). `requests` and `deliveries` contain the same aggregate fields:

| Field | Meaning |
| --- | --- |
| `active` | Not completed, with expiry later than `asOf`; request admission uses this definition |
| `ready` | Active work with payload, attempts below five, due now and no unexpired lease |
| `leased` | Active work with an unexpired lease, including a fifth in-flight claim |
| `delayed` | Active work below the attempt limit, scheduled for later, with no unexpired lease |
| `attemptedActive` | Active work with at least one persisted claim; includes initial in-flight attempts and is not a failure counter |
| `awaitingCleanup` | Uncompleted expired work, or attempt-limited work with no unexpired lease |
| `completed` | Terminal records still retained; includes successful, discarded, expired and exhausted work |
| `outcomes` | Counts of retained completed rows grouped by the fixed outcome codes below |
| `payloadRows` | All remaining encrypted payload rows, including work awaiting cleanup |
| `oldestActiveAt` | Earliest creation time among active records; null if none |
| `oldestPayloadAt` | Earliest creation time among remaining payloads; null if none |
| `latestPayloadExpiry` | Latest expiry among remaining payloads; null if none |

Counts overlap and are not a partition. For example, a leased item can also be attempted-active, and attempt-limited active work can await cleanup. No metric includes user IDs, request/challenge/session IDs, addresses, token/digest/lease values, ciphertext or database URLs.

## Operational use and limits

- Watch sustained ready backlog and age relative to `asOf`. Requests expire after fifteen minutes, so investigate before they reach that deadline. Establish thresholds from measured workload/provider behavior.
- Request active count approaching capacity means new admissions may receive a generic 503. Delayed work can reflect ordinary bounded retry backoff; it does not by itself prove an outage.
- Unexpired leases mean work may be running. Persistent cleanup backlog or old payloads warrants checking that configured API workers are running and can reach PostgreSQL. This command never performs cleanup itself.
- Payload counts and timestamps help planned key retirement. Follow [the key-rotation runbook](IDENTITY_KEY_ROTATION.md); do not infer key compatibility from counts or a successful diagnostic read, because the command does not decrypt data.
- Completed totals are not delivery-success counts. Use the bounded `outcomes` fields below; provider acceptance is not confirmed delivery to a recipient. This command remains an operator diagnostic foundation, not a complete monitoring/alerting system.

Keep diagnostic access restricted to trusted operations. Protect the database connection secret and avoid echoing it in shell history/logs. Monitoring export, alert routing, provider/failure telemetry, retention/backup exercises and independent review remain launch work. Nothing in this command authorizes public deployment.

## Retained terminal outcomes (migration 000010)

| JSON outcome field | Scope | Meaning |
| --- | --- | --- |
| `issued` | Requests | Challenge issuance, encrypted mail insertion and request consumption committed together |
| `noAction` | Requests | No new challenge was issued; unknown/disabled recipients, invalid verification sessions, already-verified accounts and resend quotas share this code |
| `accepted` | Mail | Sender reported success; the SMTP adapter uses DATA acceptance as this boundary |
| `invalidated` | Mail | Live account/proof/address/digest validation rejected pending delivery |
| `expired` | Both | Cleanup discarded expired work |
| `exhausted` | Both | The attempt limit was reached without a successful completion, including crashed claims |
| `legacyUnknown` | Both | Historical or rolling old-writer completion with no reliable recorded outcome |

All fields are present; inapplicable fields are zero. New workers write an allowlisted outcome in the same transaction that completes work and erases its encrypted payload. Stale lease acknowledgements cannot overwrite a newer result. Wrong-key/tampered envelopes remain pending without a terminal outcome until configuration is corrected or cleanup expires them. Success on the fifth attempt is still accepted/issued, not inferred exhaustion.

These are counts of retained rows, **not cumulative counters**. They decrease when one-day cleanup or account-deletion cascades remove records. Do not apply counter-rate calculations to them. Migration backfills historical completion as `LEGACY_UNKNOWN`; it never guesses from attempt counts or timestamps. During a rolling upgrade, old writers may still leave a NULL outcome, which diagnostics also reports as legacy unknown. Outcome codes include no recipient/provider reason strings, and the HTTP acceptance response remains generic.

## Prometheus text output

Run `go run ./cmd/identity-status -format=prometheus` with the same restricted database connection supplied out of band. JSON remains the default; `-format=json` is explicit. Unsupported formats/arguments fail with fixed usage text without echoing supplied values, and `-h` works without database configuration. No HTTP listener, collector installation or alert routing is added. Disabled scheduler/Prometheus templates and tested rules are available separately; nothing is activated.

All fourteen metric families are **gauges**. Label values come only from fixed queue names (`requests`, `deliveries`) and outcome names (`issued`, `no_action`, `accepted`, `invalidated`, `expired`, `exhausted`, `legacy_unknown`). The format always emits the same 38 series, including zeros for empty queues and inapplicable outcomes. No recipient, account ID, connection URL, provider response, key or ciphertext becomes a label.

| Metric | Meaning |
| --- | --- |
| `anime_identity_snapshot_timestamp_seconds` | Database snapshot time as Unix seconds; use it to detect stale exports |
| `anime_identity_requests_capacity` | Shared request admission capacity |
| `anime_identity_queue_active`, `_ready`, `_leased`, `_delayed` | Queue state counts defined above, with the `queue` label |
| `anime_identity_queue_attempted_active`, `_awaiting_cleanup`, `_completed`, `_payload_rows` | Additional retained/state counts defined above, with the `queue` label |
| `anime_identity_queue_outcome_rows` | Retained terminal counts with `queue` and `outcome` labels; never a cumulative counter |
| `anime_identity_queue_oldest_active_age_seconds` | Snapshot-relative oldest active age; zero when empty or creation lies in the future |
| `anime_identity_queue_oldest_payload_age_seconds` | Snapshot-relative oldest encrypted payload age; zero when empty or future |
| `anime_identity_queue_latest_payload_expiry_timestamp_seconds` | Latest retained payload expiry in Unix seconds; zero when empty |

For a textfile collector, write stdout to a temporary file in the destination directory and atomically rename it to the final `.prom` path only after the command exits successfully. Configure file permissions for the trusted scraper. A failed run must not replace the last complete snapshot; monitor its timestamp so old data cannot look healthy. The CLI now supports `-output-file=/absolute/path.prom` to perform same-directory staging, sync/close, 0640 mode and atomic rename itself. It requires `-format=prometheus`, an absolute `.prom` destination and an existing trusted directory. Collector activation/receiver routing remains an operator responsibility and is not installed by the command.

Interpret ages together with active/payload counts, since zero also represents empty queues. Counts describe the shared database, not an individual API process; avoid summing duplicate snapshots of the same database across collector hosts. Retention and account deletion reduce outcome counts, so use gauge comparisons and avoid counter functions such as `rate`/`increase` on these series. Provider failure timing, end-recipient delivery, missing-scrape detection and reviewed production alert thresholds still need additional monitoring design.

## Tested monitoring templates

See [deployments/monitoring](../deployments/monitoring/README.md) for inactive systemd/timer and loopback Prometheus examples plus tested alert rules. They cover marked target availability, missing/stale/future snapshots, queue capacity/age, cleanup and retained exhaustion. State alerts require fresh data; missing snapshots are detected per marked job/instance even when another target is healthy. Tests use checksum-pinned Prometheus LTS tooling and actual exporter rendering. No unit, scrape configuration or notification receiver is activated by these files.
