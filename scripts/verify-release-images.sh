#!/usr/bin/env bash
# Load a trusted release bundle into an isolated Docker daemon.
set -euo pipefail

if [[ $# != 2 || ! "$2" =~ ^[0-9a-f]{40}$ ]]; then
  echo 'usage: verify-release-images.sh <bundle-directory> <expected-commit-sha>' >&2
  exit 2
fi

commit=$2
cd -- "$1"

verify_manifest() {
  local role key value id
  local -A manifest=()
  while IFS='=' read -r key value || [[ -n "$key" || -n "$value" ]]; do
    [[ "$key" =~ ^[a-z_]+$ && -n "$value" && ! ${manifest[$key]+present} ]] || return 1
    manifest[$key]=$value
  done < manifest.txt
  [[ ${#manifest[@]} = 8 && ${manifest[commit]:-} = "$commit" && ${manifest[public_api_base]:-} = /api/v1 ]] || return 1
  for role in api web migrate; do
    id=${manifest[${role}_image_id]:-}
    [[ ${manifest[${role}_image]:-} = "anime-intelligence/$role:$commit" && "$id" =~ ^sha256:[0-9a-f]{64}$ ]] || return 1
  done
}

if ! sha256sum images.tar.gz manifest.txt | cmp --silent - SHA256SUMS; then
  echo 'Release bundle checksums do not match.' >&2
  exit 1
fi
if ! verify_manifest; then
  echo 'Release manifest does not match the expected commit and images.' >&2
  exit 1
fi

for role in api web migrate; do
  if docker image inspect "anime-intelligence/$role:$commit" >/dev/null 2>&1; then
    echo 'Expected image tags must be absent before archive verification.' >&2
    exit 1
  fi
done

docker image load --input images.tar.gz
for role in api web migrate; do
  image="anime-intelligence/$role:$commit"
  id=$(docker image inspect --format '{{.Id}}' "$image")
  revision=$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "$image")
  if [[ "$revision" != "$commit" ]] || ! grep -Fqx "${role}_image_id=$id" manifest.txt; then
    echo "Loaded $role image does not match the reviewed manifest." >&2
    exit 1
  fi
done
