#!/usr/bin/env bash
# Build and check local application images in isolated network namespaces.
set -euo pipefail

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
if [[ $# != 0 && $# != 2 ]]; then
  echo 'usage: check-containers.sh [api-image web-image]' >&2
  exit 1
fi
scratch=
api=
web=
cleanup() {
  if [[ -n "$api" ]]; then docker stop "$api" >/dev/null 2>&1 || true; fi
  if [[ -n "$web" ]]; then docker stop "$web" >/dev/null 2>&1 || true; fi
  if [[ -n "$scratch" ]]; then
    docker image rm "$api_image" "$web_image" >/dev/null 2>&1 || true
    rmdir "$scratch"
  fi
}
trap cleanup EXIT

if [[ $# == 2 ]]; then
  api_image=$1
  web_image=$2
else
  scratch=$(mktemp -d)
  suffix=${scratch##*/}
  suffix=${suffix,,}
  api_image="anime-api-smoke:$suffix"
  web_image="anime-web-smoke:$suffix"
  docker build -t "$api_image" "$repo"
  docker build -t "$web_image" "$repo/web"
fi
api=$(docker run --rm --network none -d "$api_image")
web=$(docker run --rm --network none -d "$web_image")

check() {
  local container=$1 url=$2
  test "$(docker exec "$container" id -u)" != 0
  for attempt in {1..30}; do
    if docker exec "$container" wget -Y off -qO /dev/null "$url"; then return 0; fi
    sleep 1
  done
  docker logs "$container" >&2
  return 1
}
check "$api" http://127.0.0.1:8080/readyz
check "$web" http://127.0.0.1:3000/
echo "API and web images passed non-root startup and loopback probes."
