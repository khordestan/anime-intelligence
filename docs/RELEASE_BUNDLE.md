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
