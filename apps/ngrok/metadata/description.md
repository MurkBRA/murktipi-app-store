# Ngrok Tunnel & Optional Traefik Filter

**Ngrok** exposes local services and Runtipi Traefik routes over HTTPS tunnels via **`ghcr.io/murkbra/ngrok:3.39.13`** (single lightweight Alpine container combining the official `ngrok/ngrok:3.39.13-alpine` binary with optional `nginx:1.31.10-alpine`), with the local web inspector on `${APP_PORT}` (default **`4040`**).

Because it runs as a single service, Runtipi keeps everything directly on `tipi_main_network` without creating an extra per-app IPAM subnet, `depends_on`, or host file mounts.

## Tunnel Modes (`ENABLE_NGINX_FILTER`)

### 1. With Built-in Nginx Filter Enabled (`true`, Default)
- Starts Nginx internally using `/app/default.conf` (or custom `NGINX_CONF_BASE64` supporting raw Base64 or `data:application/octet-stream;base64,...`) and runs `ngrok http --url=<NGROK_URL> 127.0.0.1:80`.
- `/` returns a clean dark `404` page, `/error.html` redirects to `/`, and all other paths (such as `/telegram`) proxy to `http://runtipi-reverse-proxy:80` with WebSocket and `X-Forwarded-*` headers preserved.

### 2. With Nginx Filter Disabled (`false`)
- Skips Nginx completely and runs `ngrok http --url=<NGROK_URL> <NGROK_TARGET>` (defaulting to `80` when `NGROK_TARGET` is blank; supports `80`, `8080`, or `container:port`).

## Routing Downstream Services via Traefik Labels

Attach standard Traefik `PathPrefix` rules on the shared Runtipi network so requests matching a specific path flow through Ngrok -> Nginx -> `runtipi-reverse-proxy:80`:

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.telegram.rule=PathPrefix(`/telegram`)"
  - "traefik.http.routers.telegram.entrypoints=web"
  - "traefik.http.routers.telegram.middlewares=telegram-strip"
  - "traefik.http.middlewares.telegram-strip.stripprefix.prefixes=/telegram"
  - "traefik.http.services.telegram.loadbalancer.server.port=3000"
```
