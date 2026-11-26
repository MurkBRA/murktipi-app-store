# Ngrok Tunnel & Optional Traefik Filter

**Ngrok** exposes local services and Runtipi Traefik routes over secure HTTPS tunnels using the official `ngrok/ngrok:3.39.13-alpine` image, with the local web inspector available on `${APP_PORT}` (default **`4040`**).

## Tunnel Modes (`ENABLE_NGINX_FILTER`)

### 1. With `ngrok-filtros` Enabled (Default)
- Runs `ngrok http [--url=<NGROK_URL>] ngrok-filtros:80`.
- The lightweight **`ngrok-filtros`** (`nginx:1.31.10-alpine`) sidecar writes `/etc/nginx/conf.d/default.conf` directly from Base64 inside the container — **no `/etc/nginx/conf.d/default.conf:ro` host volume mount is required**.
- By default, `/` returns a dark minimal `404` page, `/error.html` redirects to `/`, and all other paths (such as `/telegram`) proxy to `http://runtipi-reverse-proxy:80` with WebSocket and `X-Forwarded-*` headers preserved.
- Override `NGINX_CONF_BASE64` with either raw Base64 or a full `data:application/octet-stream;base64,...` URI if you want a custom Nginx server block.

### 2. With `ngrok-filtros` Disabled (`false`)
- Keeps the Nginx sidecar idle and forwards Ngrok directly to **`NGROK_TARGET`** (`http [--url=<NGROK_URL>] <NGROK_TARGET>`), defaulting to `80` when `NGROK_TARGET` is left blank (for example `80`, `8080`, `runtipi-reverse-proxy:80`, or `container:port`).

## Routing Downstream Services via Traefik Labels

Attach standard Traefik `PathPrefix` rules on the shared Runtipi network so requests matching a specific path flow through `ngrok-filtros` -> `runtipi-reverse-proxy:80`:

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.telegram.rule=PathPrefix(`/telegram`)"
  - "traefik.http.routers.telegram.entrypoints=web"
  - "traefik.http.routers.telegram.middlewares=telegram-strip"
  - "traefik.http.middlewares.telegram-strip.stripprefix.prefixes=/telegram"
  - "traefik.http.services.telegram.loadbalancer.server.port=3000"
```
