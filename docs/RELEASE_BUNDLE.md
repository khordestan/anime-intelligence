# Rehearse release images

The **Release bundle** workflow builds API and dashboard images from the selected commit. It bundles the two images, their image IDs, commit and checksums. The dashboard uses same-origin `/api/v1`, so a rehearsal ingress must route that path to the API and other dashboard requests to the web container.

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

The bundle contains application images. A production rehearsal needs separately provisioned PostgreSQL, schema and runtime grants, ingress and secrets. Image readiness and checksum verification are useful inputs to deployment review.
