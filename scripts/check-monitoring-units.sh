#!/bin/sh
# Parse/verify examples without installing them or requiring the release binary.
set -eu
repo_root=$(cd -- "$(dirname -- "$0")/.." && pwd)
verify_dir=$(mktemp -d)
trap 'rm -rf -- "$verify_dir"' EXIT HUP INT TERM
sed 's#^ExecStart=.*#ExecStart=/usr/bin/true#' \
  "$repo_root/deployments/monitoring/anime-identity-metrics.service" \
  > "$verify_dir/anime-identity-metrics.service"
cp "$repo_root/deployments/monitoring/anime-identity-metrics.timer" \
  "$verify_dir/anime-identity-metrics.timer"
systemd-analyze verify --man=no "$verify_dir/anime-identity-metrics.service" \
  "$verify_dir/anime-identity-metrics.timer"
