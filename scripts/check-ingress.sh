#!/usr/bin/env bash
# Exercise the loopback-only Nginx example against a synthetic upstream.
set -euo pipefail

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
image=nginx:1.30-alpine@sha256:8eb2b5540eae779603cc3db84b3986a15ecb5308b54303146eee9837c137b51a
docker run --rm --network none -i "$image" sh -eu -c '
    sed "/^http {/a\    include /tmp/anime-upstream.conf;" > /tmp/anime-ingress.conf
    cat > /tmp/anime-upstream.conf <<'"'"'CONF'"'"'
server {
    listen 127.0.0.1:8080;
    location / {
        return 200 "$http_forwarded|$http_x_forwarded_for|$http_x_real_ip";
    }
}
CONF
    nginx -t -c /tmp/anime-ingress.conf
    nginx -c /tmp/anime-ingress.conf

    unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY
    base=http://127.0.0.1:8081
    n=1
    while [ "$n" -le 6 ]; do
        result=$(wget -qO- --post-data="" --header="X-Forwarded-For: 192.0.2.$n" \
            --header="Forwarded: for=192.0.2.$n" --header="X-Real-IP: 192.0.2.$n" "$base/api/v1/auth/login")
        [ "$result" = "||" ]
        n=$((n+1))
    done
    # New untrusted forwarding headers cannot create a new request budget.
    limit=$(printf "POST /api/v1/auth/password-reset/request HTTP/1.1\r\nHost: localhost\r\nOrigin: http://localhost:3000\r\nX-Forwarded-For: 192.0.2.200\r\nContent-Length: 0\r\nConnection: close\r\n\r\n" |
        nc -w 2 127.0.0.1 8081)
    printf "%s\n" "$limit" | grep -q "^HTTP/1.1 429"
    printf "%s\n" "$limit" | grep -qi "^Retry-After: 60"
    printf "%s\n" "$limit" | grep -qi "^Access-Control-Allow-Origin: http://localhost:3000"
    # The independent catalog and assisted-analysis paths remain available.
    [ "$(wget -qO- "$base/api/v1/anime")" = "||" ]
    [ "$(wget -qO- --post-data="" "$base/api/v1/analysis/catalog/assisted")" = "||" ]
    echo "Ingress syntax, forwarding header stripping, shared identity budget and unaffected paths passed."
' < "$repo/deployments/ingress/nginx.conf"
