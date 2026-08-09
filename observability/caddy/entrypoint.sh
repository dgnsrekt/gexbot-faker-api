#!/bin/sh
set -eu

if [ "$OBSERVABILITY_AUTH_ENABLED" = "true" ]; then
	if [ -z "$OBSERVABILITY_PASSWORD" ]; then
		echo "OBSERVABILITY_PASSWORD is required when authentication is enabled" >&2
		exit 1
	fi
	hash="$(caddy hash-password --plaintext "$OBSERVABILITY_PASSWORD")"
	printf 'basic_auth {\n\t%s %s\n}\n' "$OBSERVABILITY_USERNAME" "$hash" > /tmp/auth.caddy
else
	: > /tmp/auth.caddy
fi

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
