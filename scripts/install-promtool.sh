#!/bin/sh
# CI/test dependency only. This does not install or start Prometheus.
set -eu
if [ "$#" -ne 1 ]; then
  echo 'usage: install-promtool.sh DESTINATION_DIRECTORY' >&2
  exit 1
fi
version=3.13.5
checksum=0388822ace519c5a3a8ef556a541ce1b536e21d872f82deded465bd27a5a0699
download_dir=$(mktemp -d)
trap 'rm -rf -- "$download_dir"' EXIT HUP INT TERM
archive="$download_dir/prometheus.tar.gz"
curl --fail --silent --show-error --location --max-time 60 \
  "https://github.com/prometheus/prometheus/releases/download/v$version/prometheus-$version.linux-amd64.tar.gz" \
  --output "$archive"
printf '%s  %s\n' "$checksum" "$archive" | sha256sum --check --status
tar --extract --gzip --file "$archive" --directory "$download_dir" --strip-components=1 \
  "prometheus-$version.linux-amd64/promtool"
mkdir -p -- "$1"
cp -- "$download_dir/promtool" "$1/promtool"
chmod 755 "$1/promtool"
"$1/promtool" --version
