#!/bin/sh
set -eu

CONFIG_FILE="/tmp/ngrok.yml"
{
    printf 'version: 3\n'
    printf 'agent:\n'
    printf '  web_addr: 0.0.0.0:4040\n'
    if [ -n "${NGROK_AUTHTOKEN:-}" ]; then
        printf '  authtoken: "%s"\n' "$NGROK_AUTHTOKEN"
    fi
} > "$CONFIG_FILE"

cleanup() {
    if [ -f /run/nginx.pid ]; then
        nginx -s quit >/dev/null 2>&1 || true
    fi
}
trap cleanup INT TERM

FILTER_ENABLED="$(printf '%s' "${ENABLE_NGINX_FILTER:-true}" | tr '[:upper:]' '[:lower:]')"

if [ "$FILTER_ENABLED" = "true" ]; then
    if [ -n "${NGINX_CONF_BASE64:-}" ]; then
        printf '%s' "$NGINX_CONF_BASE64" \
            | sed 's|^data:[^,]*,||' \
            | tr -d ' \t\r\n' \
            | base64 -d \
            | tr -d '\r' > /etc/nginx/conf.d/default.conf
    else
        cp /app/default.conf /etc/nginx/conf.d/default.conf
    fi
    nginx -t
    nginx
    TARGET="127.0.0.1:80"
else
    TARGET="${NGROK_TARGET:-80}"
fi

if [ "$#" -gt 0 ]; then
    exec ngrok --config="$CONFIG_FILE" "$@"
fi

if [ -n "${NGROK_URL:-}" ]; then
    exec ngrok --config="$CONFIG_FILE" http --url="$NGROK_URL" "$TARGET"
else
    exec ngrok --config="$CONFIG_FILE" http "$TARGET"
fi
