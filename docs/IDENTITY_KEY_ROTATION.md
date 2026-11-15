# Identity queue key rotation

The request inbox and mail outbox use AES-256-GCM with keys held outside PostgreSQL. Rotation does not change database schema or ciphertext formats. All API instances currently run queue workers; the standalone worker command remains a placeholder.

## Configuration

- `MAIL_OUTBOX_KEY`: required primary secret, a canonical base64-encoded 32-byte key. Every new request or delivery envelope uses this key.
- `MAIL_OUTBOX_PREVIOUS_KEYS`: optional comma-separated decryption keys, at most three. Supply canonical base64 values without whitespace, empty entries or duplicates. None may equal the primary.
- Missing/invalid primary, malformed/duplicate/excess decryption keys, or partial mail configuration stops startup with a fixed error. Values are never logged.

The primary and additional keys can decrypt both queue types, but only the primary encrypts. Authenticated domains and immutable row binding remain enforced on every key attempt. Trying at most four keys bounds decryption work. Unknown-key/tampered envelopes fail closed without erasing pending work; an incompatible node can block at an unreadable head row until its configuration is corrected or the row expires.

## Planned rolling rotation

Use separate secret-manager entries for healthy current key A and newly generated key B. These labels describe secrets, not literal configuration values. Do not put key values in Git, logs, tickets, screenshots or database rows.

| Phase | Primary on each node | Additional decryption keys | Condition before advancing |
| --- | --- | --- | --- |
| Prepare readers | A | B, plus any still-required older keys | Every node can read B before any node writes B |
| Switch writers | B | A, plus required older keys | Every node has switched; no old-primary process or in-flight writer remains |
| Retire A | B | Only keys still required by pending work | All possible A-encrypted inbox/outbox payloads have completed or expired and been erased |

Keep the total within the four-key limit at every phase; retire drained older material before beginning another rotation if necessary. Secret delivery and process rollout are operational responsibilities; the application does not distribute or generate production keys automatically.

Because legacy ciphertext has no key identifier, queue counts cannot classify rows by encryption key. The simplest retirement procedure is to pause new admission, keep compatible workers running until both queues drain/expire and clear all payloads, then remove A. For uninterrupted admission, record a database-time cutoff after all A writers have stopped. Retain A until every payload row created on or before that cutoff has been consumed or expired and cleared. This conservatively includes some B-written rows but excludes no A-written row. Never infer readiness from an empty ready-to-send list: leased, delayed or temporarily unreadable work still matters.

An operator using an authorized administrative connection can inspect counts without reading private envelopes:

```sql
SELECT 'request_inbox' AS queue,
       count(*) FILTER (WHERE payload IS NOT NULL) AS encrypted_rows,
       max(expires_at) FILTER (WHERE payload IS NOT NULL) AS latest_expiry
FROM identity_requests
UNION ALL
SELECT 'mail_outbox',
       count(*) FILTER (WHERE payload IS NOT NULL),
       max(expires_at) FILTER (WHERE payload IS NOT NULL)
FROM identity_outbox;
```

For the cutoff method, apply the same recorded `created_at <= cutoff` predicate to both table queries. Use parameterized operator tooling; do not copy private payloads into diagnostics. Requests have fifteen-minute deadlines and mail proofs last at most one hour from issuance, but expiry alone does not erase rows while workers are stopped. Confirm payload erasure before retirement. Completed rows with NULL payload need no old decryption key.

## Rollback and limitations

A rollback to A must keep B as an additional decryption key until all B-written work has cleared. A newly deployed B-only node cannot read pending A data; restore A in its bounded decryption list to recover that work. Authentication/redemption always checks live account/proof state; rotation must never weaken those checks or issue replacement proofs merely because decryption failed.

Keep key backups separate from database backups and apply a reviewed retention/access policy. Restore exercises must include compatible queue keys or deliberate expiry/discard of unrecoverable operational work. This compatibility procedure is for planned rotation of healthy keys. A suspected compromise requires an incident response that reviews pending requests/proofs and affected sessions; retaining a compromised key for compatibility alone is not a recovery strategy.

Queue monitoring, rollout tooling, backup/restore exercises and independent security review remain public-launch prerequisites. No public deployment is performed by this runbook.
