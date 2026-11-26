# Camoufox Playwright Server

**Camoufox** is a stealth, anti-detect Firefox browser fork with engine-level C++ fingerprint spoofing. Built on a lightweight **`python:3.12-slim`** Debian image wrapping the official `daijro/camoufox` (`0.5.9`) Playwright server, this container runs headless automation for **Playwright**, **Node-RED**, and **n8n** without VNC/Selkies desktop overhead.

## Connecting via Playwright (`firefox.connect`)

Playwright WebSocket automation and status endpoints listen on `${APP_PORT}` (default **`9223`**):

```js
// Node.js / Node-RED (Playwright)
const browser = await playwright.firefox.connect("ws://<host>:9223/");
const page = await browser.newPage();
await page.goto("https://example.com");
await browser.close();
```

If `WS_TOKEN` is set, pass the token via path (`ws://<host>:9223/<token>`), query parameter (`ws://<host>:9223/?token=<token>`), or `Authorization: Bearer <token>`. Opening `/` in a browser shows live connection details and startup status, while `/json` and `/health` provide endpoint discovery and readiness checks.

## Persistent `/config` Cache & Key Options

- **Persistent `/config` Cache (`${APP_DATA_DIR}/data/config:/config`):** Keeps the Docker image slim by downloading the pinned Camoufox binary (and GeoIP database only when `GEOIP=true`) on first startup.
- **Headless Mode (`HEADLESS`):** Defaults to `virtual`, using Camoufox's built-in `Xvfb` virtual display; set `true` for native headless without `Xvfb`.
- **Slim Fonts (`SLIM_FONTS`):** Optional flag (`true`) that prunes large (>5 MB) CJK and Apple Emoji font files inside `/config/.cache` to save ~1.6 GB of disk space.
- **Permissions (`PUID` / `PGID`):** Optional host UID/GID mapping for `/config` files.
- **GeoIP & Fingerprint Controls:** Optional `GEOIP`, `OS_TARGET` (`windows`/`macos`/`linux`), `LOCALE`, `BLOCK_IMAGES`, `BLOCK_WEBRTC`, `DISABLE_COOP`, and `PROXY_SERVER` / `PROXY_USERNAME` / `PROXY_PASSWORD`.
- **Shared Memory (`SHM_SIZE`):** Defaults to `1gb` for stable browser rendering.
