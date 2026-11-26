#!/usr/bin/env bash
# Build an archive for an isolated deploy rehearsal. No registry push or activation.
set -euo pipefail

if [[ $# != 1 || -z "$1" || -e "$1" || -L "$1" ]]; then
  echo 'usage: package-release-images.sh <new output directory>' >&2
  exit 1
fi
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
commit=$(git -C "$repo" rev-parse --verify HEAD)
if [[ -n "$(git -C "$repo" status --porcelain)" ]]; then
  echo 'Package from a clean source checkout.' >&2
  exit 1
fi

output="$1"
scratch=$(mktemp -d "$(dirname -- "$output")/.anime-release.XXXXXXXX")
trap 'if [[ -d "$scratch" ]]; then rm -rf -- "$scratch"; fi' EXIT
api="anime-intelligence/api:$commit"
web="anime-intelligence/web:$commit"
migrate="anime-intelligence/migrate:$commit"

docker build --label "org.opencontainers.image.revision=$commit" -t "$api" "$repo"
docker build --label "org.opencontainers.image.revision=$commit" \
  --build-arg NEXT_PUBLIC_API_BASE_URL=/api/v1 -t "$web" "$repo/web"
docker build --target migrator --label "org.opencontainers.image.revision=$commit" -t "$migrate" "$repo"
api_id=$(docker image inspect --format '{{.Id}}' "$api")
web_id=$(docker image inspect --format '{{.Id}}' "$web")

migrate_id=$(docker image inspect --format '{{.Id}}' "$migrate")
docker image save "$api" "$web" "$migrate" | gzip -n > "$scratch/images.tar.gz"
{
  printf 'commit=%s\n' "$commit"
  printf 'api_image=%s\napi_image_id=%s\n' "$api" "$api_id"
  printf 'web_image=%s\nweb_image_id=%s\n' "$web" "$web_id"
  printf 'migrate_image=%s\nmigrate_image_id=%s\n' "$migrate" "$migrate_id"
  printf 'public_api_base=/api/v1\n'
} > "$scratch/manifest.txt"
(cd "$scratch" && sha256sum images.tar.gz manifest.txt > SHA256SUMS)

# Create the final directory only after all images and their checksums exist.
mv -T --no-clobber -- "$scratch" "$output"
test ! -d "$scratch"
