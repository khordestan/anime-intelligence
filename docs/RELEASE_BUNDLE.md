# Rehearse release images

The **Release bundle** workflow builds API, dashboard and migrator images from the selected commit. It bundles the three images, their image IDs, commit and checksums. The dashboard uses same-origin `/api/v1`, so a rehearsal ingress must route that path to the API and other dashboard requests to the web container.

An operator can run the workflow manually to retain a seven-day GitHub artifact. Prepare the bundle locally from a clean checkout with Docker.

```sh
scripts/package-release-images.sh /tmp/anime-review-release
```

Verify the archive in a disposable Docker environment. Set `APPROVED_COMMIT` to the independently reviewed 40-character commit SHA.

```sh
scripts/verify-release-images.sh /tmp/anime-review-release "$APPROVED_COMMIT"
```

The verifier checks both archive files, the manifest, each loaded image ID, and its `org.opencontainers.image.revision` label against the supplied commit. Use a disposable Docker daemon where the expected tags are absent. The hosted workflow exercises this round trip and checks non-root startup and same-origin ingress routing from the loaded images. The ingress exercise runs in an isolated network namespace. Keep the archive and checksum file from the same trusted workflow run.

The standalone migrator runs as a non-root user and requires an explicitly supplied `MIGRATION_DATABASE_URL`. The hosted rehearsal applies the same migrations twice in an isolated PostgreSQL container, provisions a synthetic restricted runtime role, and checks API readiness. A production rehearsal needs separately provisioned PostgreSQL, approved migration and runtime credentials, ingress and secrets. Keep the migration credential out of long-running API containers. Image readiness and checksum verification are useful inputs to deployment review.

Set `REQUIRE_POSTGRES=true` on the hosted API so missing database configuration stops startup before the service binds a port. Provide the restricted runtime connection through `DATABASE_URL`.

## Exercise the synthetic dashboard workload

Set `TEST_DATABASE_URL` to a disposable PostgreSQL administration connection and run:

```sh
ANIME_LOAD_EXTENDED=1 go test -count=1 -v ./internal/platform/postgres -run '^TestConcurrentDashboardWorkload$'
```

The default workload uses eight verified synthetic owners and 1,000 titles. The extended workload uses 24 owners and 10,000 titles. Both run two API replicas with independent connection pools and check that each competing pair of progress updates commits exactly once. The report includes request counts and client-side p50 and p95 latency by route. Use a comparable environment and fixture when comparing runs. Production workload and capacity still require environment-specific measurements.

The PostgreSQL reliability test `TestDashboardProgressRetryAfterReplicaResponseLoss` drops an HTTP response after the first replica commits, then takes that replica offline. The second replica remains ready, rejects the stale retry and accepts the next update after the dashboard rereads progress.

## Plan a controlled rollout

The runbook assumes a separately authorized environment with private database access, a deployment operator and an approved application commit. Rehearse the sequence on disposable or authorized staging infrastructure before scheduling a production change.

1. Record the candidate and previous application commits, their workflow runs, image digests, configuration versions and the database migration ledger. Retain both verified image bundles so application rollback can use the previous artifact.
2. Confirm that PostgreSQL is recoverable under the environment's recovery objectives. Record the backup reference and recovery point alongside the change. Verify that required encryption keys and the post-snapshot account reconciliation procedure are available to the recovery operator. [Backup and recovery](BACKUP_RECOVERY.md)
3. Rehearse the candidate migration and role grants against an isolated copy of the supported schema. Test the previous application against the migrated schema with the intended grants if the rollout requires overlapping replicas or application rollback. When overlap is incompatible, schedule a maintenance window and fence writers before the migration.
4. Run the candidate bundle's one-shot migrator with the separately provisioned migration credential. Refresh the candidate runtime grants using the SQL from the same commit. Recheck the migration ledger and runtime role before starting application replicas. The API credential belongs only to the runtime.
5. Start a candidate API replica with `REQUIRE_POSTGRES=true` and its matching dashboard. Check API liveness, database-backed readiness and the dashboard's same-origin API route from within the approved environment. Verify that configured routes work through the intended ingress and that monitoring observes the candidate.
6. Move traffic only after those checks pass. Compare readiness, request errors and latency to the environment's acceptance thresholds. Check queue freshness when delivery is already enabled. Verify a read and a retry with a dedicated test account. Retain the previous images and record the final commit, ledger and checks.

If a migration fails, keep traffic on the last verified compatible application or keep it fenced. Compare the ledger with the rehearsal and inspect the failed migration before retrying; earlier migrations in the same run may have applied. If an application rollout fails after migration, route traffic to the previously verified application only when the schema and grants remain compatible. Database downgrade or restore requires its own reviewed procedure and recovery decision. Keep delivery workers fenced during recovery until account reconciliation and credential fencing are complete. [PostgreSQL runtime](POSTGRES_RUNTIME.md) · [Backup and recovery](BACKUP_RECOVERY.md)
