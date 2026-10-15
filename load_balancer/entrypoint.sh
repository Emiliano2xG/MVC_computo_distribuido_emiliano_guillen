#!/bin/sh
set -e

CONSUL_ADDR="${CONSUL_HTTP_ADDR:-consul:8500}"

exec consul-template \
  -log-level=INFO \
  -consul-addr="$CONSUL_ADDR" \
  -template="/etc/consul-template/nginx.conf.ctmpl:/etc/nginx/conf.d/default.conf:nginx -s reload 2>/dev/null || true" \
  -exec="nginx -g 'daemon off;'"
