#!/usr/bin/env bash
# Rehearse same-origin routing without exposing any host ports.
set -euo pipefail
if [[ $# != 2 ]]; then
  echo 'usage: check-bundle-ingress.sh <api-image> <web-image>' >&2
  exit 1
fi
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
scratch=$(mktemp -d)
api=
web=
ingress=
cleanup() {
  if [[ -n "$ingress" ]]; then docker rm -f "$ingress" >/dev/null 2>&1 || true; fi
  for container in "$web" "$api"; do
    if [[ -n "$container" ]]; then docker stop "$container" >/dev/null 2>&1 || true; fi
  done
  rm -rf -- "$scratch"
}
trap cleanup EXIT
install -m 0644 "$repo/deployments/ingress/bundle-loopback.conf" "$scratch/nginx.conf"

api=$(docker run --rm --network none -d "$1")
web=$(docker run --rm --network "container:$api" -d "$2")
ingress=$(docker run --network "container:$api" --user nginx -d \
  --mount "type=bind,src=$scratch/nginx.conf,dst=/etc/nginx/nginx.conf,readonly" \
  --entrypoint nginx nginx:1.30-alpine -g 'daemon off;')

check() {
  local url=$1 marker=$2 body
  for attempt in {1..30}; do
    if body=$(docker exec "$ingress" wget -Y off -qO - "$url" 2>/dev/null) &&
      [[ "$body" == *"$marker"* ]]; then
      return 0
    fi
    sleep 1
  done
  docker logs "$ingress" >&2
  return 1
}
check http://127.0.0.1:8090/ 'Anime Intelligence'
check http://127.0.0.1:8090/api/v1 '"authenticationRequired"'
check 'http://127.0.0.1:8090/api/v1/anime?limit=1' '"items"'
echo 'The bundle dashboard and API answered on the same isolated origin.'
