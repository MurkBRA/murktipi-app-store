# Camoufox Playwright Server

**Camoufox** is a stealth, anti-detect Firefox browser fork with engine-level C++ fingerprint spoofing. This container packages the official `daijro/camoufox` Playwright WebSocket server (`0.5.9`) together with an integrated **Selkies** live HTML5 browser viewer (`/viewer/`), a status dashboard (`/`), `/json` endpoint discovery, and optional token authentication on a single port (`9223`) for **Playwright**, **Node-RED**, and **n8n** workflows.

## Live Browser Viewer (`/viewer/`)

When `HEADLESS=virtual` (the default) and `ENABLE_VIEWER` is enabled (`true` by default), Camoufox runs inside a shared virtual X11 display (`:99`, Openbox) streamed by **Selkies**.

1. Click **Open** in Runtipi to open the Camoufox dashboard on port `9223`.
2. Click **Open Live Browser Viewer** (or navigate directly to `http://<host>:<port>/viewer/`).
3. Any browser context or tab opened by Playwright (`firefox.connect`) renders live in `/viewer/`, with full mouse and keyboard interaction.

> If `WS_TOKEN` is configured, protect `/viewer/` using your token via `?token=<token>` (sets an HttpOnly session cookie), HTTP Basic Auth (password = `WS_TOKEN`), or an `Authorization: Bearer <token>` header.

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

- **Display Mode (`HEADLESS`):** Defaults to `virtual` (Xvfb virtual X11 display + Selkies viewer) with optional lightweight `true` native headless mode (disables the viewer to save RAM).
- **Live Viewer Controls (`ENABLE_VIEWER`, `VIEWER_RESOLUTION`):** Keep the Selkies stream enabled in `virtual` mode (default `true`) or disable it (`false`), and set initial virtual screen dimensions (default `1920x1080`).
- **Shared Memory (`SHM_SIZE`):** Defaults to `256m` for virtual display stability; raise to `512m` or `1g` for heavy multi-tab automation.
- **GeoIP & Fingerprint Controls:** Optional `GEOIP`, `OS_TARGET` (`windows`/`macos`/`linux`), `LOCALE`, `BLOCK_IMAGES`, `BLOCK_WEBRTC`, and `DISABLE_COOP` pass-through flags.
- **Upstream Proxy:** Optional `PROXY_SERVER`, `PROXY_USERNAME`, and masked `PROXY_PASSWORD` settings.
- **Status Dashboard & Healthcheck:** Opening the app UI displays Playwright connection examples, live Selkies viewer access, active WebSocket clients, and readiness (`/health` and `/json`).
