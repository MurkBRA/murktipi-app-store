# Camoufox Playwright Server

**Camoufox** is a stealth, anti-detect Firefox browser fork with engine-level C++ fingerprint spoofing. This container packages the official `daijro/camoufox` Playwright WebSocket server (`0.5.9`) behind a lightweight HTTP status page, `/json` endpoint discovery, and optional token authentication for **Playwright**, **Node-RED**, and **n8n** workflows.

## Connecting via Playwright (`firefox.connect`)

By default, Camoufox listens on host port `9223` (`${APP_PORT}:9223`):

```js
// Node.js / Node-RED (Playwright)
const browser = await playwright.firefox.connect("ws://<host>:9223/");
const page = await browser.newPage();
await page.goto("https://example.com");
await browser.close();
```

If `WS_TOKEN` is set, pass the token via path (`ws://<host>:9223/<token>`), query parameter (`ws://<host>:9223/?token=<token>`), or `Authorization: Bearer <token>`.

## Key Features & Optional Settings

- **Display Mode (`HEADLESS`):** Defaults to `virtual` (Xvfb virtual X11 display for stronger fingerprint fidelity) with optional lightweight `true` headless mode.
- **GeoIP & Fingerprint Controls:** Optional `GEOIP`, `OS_TARGET` (`windows`/`macos`/`linux`), `LOCALE`, `BLOCK_IMAGES`, `BLOCK_WEBRTC`, and `DISABLE_COOP` pass-through flags.
- **Upstream Proxy:** Optional `PROXY_SERVER`, `PROXY_USERNAME`, and masked `PROXY_PASSWORD` settings.
- **Status Dashboard & Healthcheck:** Opening the app UI displays connection examples, active WebSocket clients, and readiness (`/health` and `/json`).
