# Browserless (Headless Browser Server)

**Browserless v2** (`v2.64.3`) headless browser automation server for **Puppeteer**, **Playwright**, and **Chrome DevTools Protocol (CDP)**, with base browser image selection in the Runtipi installer.

---

## Available Base Images (`ghcr.io/browserless/*:v2.64.3`)

During installation you can pick which public Browserless image to run (**`chrome`** is pre-selected by default):

- **`chrome`** *(default)* — Official Google Chrome.
- **`chromium`** — Multi-architecture Chromium.
- **`firefox`** — Mozilla Firefox.
- **`webkit`** — WebKit (Safari engine).
- **`multi`** — Combined bundle (Chromium + Firefox + WebKit).

> **Note:** The `edge` and `sdk` variants in the Browserless repository do not provide anonymous public GHCR packages (`401/403`), so only the 5 public multi-arch (`amd64`/`arm64`) images are listed.

---

## Optional Variables & `shm_size`

All environment variables (except `TZ`, automatically injected by Runtipi) and `SHM_SIZE` are **optional** and define no forced defaults:

- **When left blank:**
  - Environment variables (`MAX_CONCURRENT_SESSIONS`, `CONNECTION_TIMEOUT`, `PREBOOT_CHROME`, `CHROME_REFRESH_TIME`, `ENABLE_CORS`, `DEMOING`) are **not injected into the container** (YAML pass-through without `=`).
  - `SHM_SIZE` uses `64m`, which matches Docker's native `/dev/shm` default when `shm_size` is omitted.
- **Example values:**
  - `SHM_SIZE`: `2gb`
  - `MAX_CONCURRENT_SESSIONS`: `10`
  - `CONNECTION_TIMEOUT`: `60000`
  - `PREBOOT_CHROME`: `true`
  - `CHROME_REFRESH_TIME`: `3600000`
  - `ENABLE_CORS`: `true`
  - `DEMOING`: `false`

---

## Connection & Ports

- **Default external port:** `9222` (mapped via `${APP_PORT}:3000`)
- **Internal endpoint on `tipi_main_network`:** `http://browserless:3000` / `ws://browserless:3000`
- **External / host endpoint:** `http://<RUNTIPI-IP>:9222` / `ws://<RUNTIPI-IP>:9222`
