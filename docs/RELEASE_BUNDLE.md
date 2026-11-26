# Rehearse release images

The **Release bundle** workflow builds API, dashboard and migrator images from the selected commit. It bundles the three images, their image IDs, commit and checksums. The dashboard uses same-origin `/api/v1`, so a rehearsal ingress must route that path to the API and other dashboard requests to the web container.

An operator can run the workflow manually to retain a seven-day GitHub artifact. Prepare the bundle locally from a clean checkout with Docker.

```sh
scripts/package-release-images.sh /tmp/anime-review-release
```

Verify the archive in a disposable Docker environment.

```sh
cd /tmp/anime-review-release
sha256sum --check SHA256SUMS
docker image load --input images.tar.gz
```

Compare each loaded image ID and its `org.opencontainers.image.revision` label with the manifest and the approved commit. The hosted workflow exercises this round trip and checks non-root startup and same-origin ingress routing from the loaded images. The ingress exercise runs in an isolated network namespace. Keep the archive and checksum file from the same trusted workflow run.

The standalone migrator runs as a non-root user and requires an explicitly supplied `MIGRATION_DATABASE_URL`. The hosted rehearsal applies the same migrations twice in an isolated PostgreSQL container, provisions a synthetic restricted runtime role, and checks API readiness. A production rehearsal needs separately provisioned PostgreSQL, approved migration and runtime credentials, ingress and secrets. Keep the migration credential out of long-running API containers. Image readiness and checksum verification are useful inputs to deployment review.

Set `REQUIRE_POSTGRES=true` on the hosted API so missing database configuration stops startup before the service binds a port. Provide the restricted runtime connection through `DATABASE_URL`.
