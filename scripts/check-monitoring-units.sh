#!/bin/sh
# Parse/verify examples without installing them or requiring the release binaries.
set -eu
repo_root=$(cd -- "$(dirname -- "$0")/.." && pwd)
verify_dir=$(mktemp -d)
trap 'rm -rf -- "$verify_dir"' EXIT HUP INT TERM
for collector in identity release-email; do
  unit=anime-$collector-metrics
  sed 's#^ExecStart=.*#ExecStart=/usr/bin/true#' \
    "$repo_root/deployments/monitoring/$unit.service" > "$verify_dir/$unit.service"
  cp "$repo_root/deployments/monitoring/$unit.timer" "$verify_dir/$unit.timer"
done
systemd-analyze verify --man=no "$verify_dir"/*.service "$verify_dir"/*.timer
