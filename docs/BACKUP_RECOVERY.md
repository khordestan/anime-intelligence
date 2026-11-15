# Backup and isolated recovery

This is a tested logical-recovery procedure for the current development schema,
not an activated backup service or permission to deploy. All exercises use
synthetic accounts and disposable PostgreSQL 17 databases. Production backup
storage, encryption, retention, deletion reconciliation, operator access,
point-in-time recovery (PITR), recovery point/time objectives and independent
security review still require operational design and approval.

## What a backup contains

A full database archive includes private product data, password hashes, bearer
and proof digests, lockouts, encrypted request/mail envelopes and the migration
ledger. Treat the whole archive as sensitive even though queue payloads are
encrypted. Restrict temporary files/directories to their operator, encrypt the
archive with reviewed backup tooling before storage/transfer, and keep backup
and decryption-key access separate. Do not upload archives to Git, CI artifacts,
issues or chat. Account export is intentionally narrower and cannot restore the
service.

Use PostgreSQL 17 `pg_dump`/`pg_restore` clients against PostgreSQL 17. Supply
credentials with operator-managed libpq service/passfile configuration, never
literal secrets in shell commands. A restricted API role cannot make a complete
backup because of forced RLS. Use a separately authorized backup administrator
with sufficient read/RLS-bypass privileges; do not elevate the API role.

A logical dump supplies a consistent database snapshot, not later writes or
cluster roles/passwords/configuration. Record the snapshot/recovery boundary,
application commit, PostgreSQL version, migration version, integrity digest,
backup-encryption reference and required queue-key references in protected backup
inventory. Preserve the keys that encrypted pending work at that boundary; the
current writer key alone may be insufficient after rotation. Never store keys
in the dump or inventory itself. See [key rotation](IDENTITY_KEY_ROTATION.md).

For example, after setting up an operator-only directory and private service
configuration (the service names below are placeholders):

```sh
umask 077
PGSERVICE=anime_backup_source pg_dump --no-password --format=custom \
  --no-owner --no-privileges --file=synthetic-or-protected.dump
```

Archive encryption, authenticated integrity checking, off-host storage, retention
and restore authorization are separate required operational steps. Neither this
example nor CI installs a scheduler or processes a real backup.

## Restore while disconnected

1. Stop all API instances, identity workers, scheduled commands and other writers.
   Keep the restored database isolated from users and outbound SMTP. Do not run
   the API just to inspect the restore: it owns the identity workers and can send
   mail as soon as configured. Prevent connection pools from reconnecting to the
   target. Select a new empty database on an isolated recovery cluster; never
   restore over the source.
2. Verify/decrypt the archive using the protected inventory and approved backup
   tooling. Recreate the migration administrator and a separate non-owner,
   non-elevated runtime role with **new** credentials from the secret manager.
   Do not restore cluster-wide roles/passwords blindly from another environment.
3. Restore using the administrator with `--no-owner --no-privileges
   --single-transaction --exit-on-error`. The target should start from `template0`;
   its ownership and ACLs are recovery-environment choices. A failure must leave
   the target quarantined. Never execute an archive from an untrusted source.
4. Check the restored migration ledger/checksums with `cmd/migrate` using the
   matching application commit and migration administrator. Investigate drift;
   do not edit stored checksums to force acceptance. Apply the reviewed
   `deployments/runtime-role.sql` grants to the separate runtime role and
   provision its password out of band. The archive deliberately excludes ACLs.
5. Select the recovery policy below **before** admitting users or enabling mail.
   Validate restricted-role startup/readiness, forced RLS and owner-scoped
   queries with synthetic checks. `/readyz` checks only minimum live safeguards;
   it does not prove complete policy correctness or operational readiness.

A restore example, with an explicitly selected empty target service:

```sh
PGSERVICE=anime_isolated_restore pg_restore --no-password --no-owner \
  --no-privileges --single-transaction --exit-on-error synthetic-or-protected.dump
```

### Fenced disaster recovery (default)

A historical restore can resurrect revoked sessions, old passwords, used proofs,
deleted accounts, outdated email ownership and queued mail. Database checksums
and a green readiness probe cannot identify those later changes.

While all writers are stopped, run the manual script against the isolated target:

```sh
PGSERVICE=anime_isolated_restore psql -X --no-password -v ON_ERROR_STOP=1 \
  --file=deployments/recovery-fence.sql
```

This administrator-only transaction takes exclusive locks, sets active accounts'
`locked_until` to PostgreSQL infinity, revokes all sessions and unconsumed proofs,
and **deletes every restored identity request and mail envelope**. It preserves
product data, disabled-account state and consumed proof history. It needs no
queue decryption keys. Errors roll back the entire transaction; the psql process
must exit on failure. Five-second lock and thirty-second statement deadlines
bound lock waits and each statement. Repeating the script while offline is safe;
running it after reopening would lock accounts again and discard new work. It is
not a migration, does not discover its target, and must never run on a live source.

Before reopening even password-recovery intake, reconcile post-backup deletions,
address changes, disables, verification changes and other security-relevant events
using an independently protected audit/deletion inventory. Delete or quarantine
affected restored records as appropriate; do not infer that an old email address
is still the correct owner. The current application does **not** maintain that
independent inventory. If it is missing or incomplete, keep the service isolated
and use a reviewed manual reconciliation process. Fencing alone cannot prevent a
deleted account from recovering through a resurrected address.

Once reconciliation and review are complete, configure new runtime/SMTP secrets
and a fresh primary queue key. Fresh password-reset proofs can unlock individual
active accounts; a successful reset clears the infinite lock and revokes sessions
again. A reset does not verify an unverified email or enable a disabled account.
Old passwords/bearers/proofs must remain rejected. Check product data isolation,
queue diagnostics and freshness alerts, then explicitly authorize traffic/mail
restoration. Do not promise a fixed recovery time until this is measured under
representative volume. A compromised backup/key requires a separate incident
response, not automatic use of this procedure.

### Compatible-key continuation (exercise only)

The drill also verifies that the exact snapshot can resume pending encrypted work
with the compatible keyring: a new key alone fails closed and preserves payloads;
adding the old key permits processing without changing issued proof tokens.
Retries may resend messages accepted after the backup boundary, and proofs or
sessions consumed/revoked after it can become valid again. This branch is tested
to establish restore fidelity and key dependencies; it is **not** the default
reopening policy. Never point a drill at real SMTP. Restoring queue keys cannot
reconcile lost account/security history.

## Automated synthetic drill

The opt-in Go test creates a fresh randomly named source database and restricted
role, seeds only synthetic accounts/resources, dumps it to a private temporary
archive, and restores it into two different empty databases. It never dumps or
restores the database named by `TEST_DATABASE_URL`; that connection is only the
disposable provisioning administrator. Generated databases/roles/archive files
are cleaned up. Do not use a production cluster as the provisioning service.

With matching PostgreSQL clients on `PATH` and a disposable PostgreSQL 17
administrator URL supplied privately:

```sh
TEST_POSTGRES_RECOVERY=1 go test -race -count=1 -timeout=3m \
  -run '^TestBackupRestoreRecovery$' ./internal/platform/postgres
```

CI's `recovery` job uses PostgreSQL 17 for both clients and server. It verifies:

- Migration ledger/checksums, restricted startup/readiness, production runtime
  grants, all eight nonempty export resources and direct forced-RLS isolation
  on all thirteen private tables after restore.
- Snapshot password/session fidelity, pre-backup session revocation, and the
  historical resurrection hazard after a later password change/account deletion.
- Missing-key preservation, compatible-key inbox/mail resumption and single-use
  proof redemption, using an in-memory sender only.
- Fencing privilege rejection, full rollback after an injected mid-script error,
  repeatability, rejection of historical passwords/bearers/proofs, queue erasure,
  deletion reconciliation and disabled-account recovery rejection.
- A fresh reset using only a new key unlocks its own account while preserving
  product data and keeping other historical credentials blocked.

Without `TEST_POSTGRES_RECOVERY=1`, this test skips explicitly. When enabled, a missing
`TEST_DATABASE_URL` or missing client fails the drill; ordinary integration tests
still skip without their database configuration. CI explicitly enables the drill. No dumps, credentials or queue keys are published as
artifacts. This logical drill does not test physical backups, WAL replay, storage
failure, secret-manager availability, real delivery, production volume or a live
recovery objective.
