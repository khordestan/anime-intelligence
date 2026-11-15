# Identity queue diagnostics

Use `go run ./cmd/identity-status` with `DATABASE_URL` supplied out of band for the restricted runtime role. The command requires the current migrated schema (through 000009) and normal runtime grants. It rejects owner/superuser connections using the same guard as the API. It does not need SMTP settings or encryption keys, contact a mail provider, start workers, decrypt envelopes, or expose an HTTP endpoint.

The command reads both queues from a single repeatable-read, read-only transaction and writes one versioned JSON object to stdout. Errors print fixed messages to stderr without connection details. Exit 0 means the snapshot was read and encoded successfully; exit 1 means missing/invalid configuration, database/query failure or output failure. Exit status is not a delivery-health verdict.

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
- Completed counts are not success/failure statistics. The current schema does not retain a terminal outcome code; proving mail-provider acceptance requires additional instrumentation that excludes private data. This command is an operator diagnostic foundation, not a complete monitoring/alerting system.

Keep diagnostic access restricted to trusted operations. Protect the database connection secret and avoid echoing it in shell history/logs. Monitoring export, alert routing, provider/failure telemetry, retention/backup exercises and independent review remain launch work. Nothing in this command authorizes public deployment.
