# Email OAuth2 Proxy UI

Lightweight **Go Web UI** (multi-arch `amd64`/`arm64` image published via GitHub Actions at `ghcr.io/murkbra/email-oauth2-proxy-ui`) to manage an existing **[Email OAuth2 Proxy](https://github.com/simonrob/email-oauth2-proxy)** installation on **Runtipi**.

---

## Key Features

- **1-Click OAuth Wizard & External Client Monitor:** Triggers IMAP/TLS login directly against the proxy and detects pending OAuth requests from external clients (MailCraft, Thunderbird, printers, etc.) with automatic `/oauth/callback` handling.
- **Real-Time Dashboard & Port Probes:** Tests configured IMAP/SMTP/POP ports and displays token status stored in `credstore.config`.
- **Provider Templates & Advanced Flows:** Ready-to-use presets for **Outlook / Office 365**, **Gmail**, **Yahoo**, **AOL**, and **Fastmail** (Mozilla Thunderbird, FairEmail, GNOME Online Accounts, Mailspring) supporting Authorization Code, Device Flow, Client Credentials (CCG), ROPCG, Google Service Account, and JWT certificates.
- **TLS Certificate Generator & `ca-cert.crt` Download:** Generates a local X.509 v3 CA + SAN server certificate, applies TLS across all servers in 1 click, and provides `ca-cert.crt` download.
- **Docker & Compose Port Manager:** Checks live host/Docker port availability via `/var/run/docker.sock`, supporting Compose YAML updates in `user-config` (Mode A) and/or live Docker container recreation (Mode B).
- **INI Config Editor, Live Debug (`SIGUSR1`) & Log Streaming:** Structured and raw editing with automatic `.bak` backups plus live `/config/emailproxy.log` streaming.

---

## Automatic Runtipi Integration (`ROOT_FOLDER_HOST`)

In Runtipi v4, **`ROOT_FOLDER_HOST`** is automatically injected into every app's `app.env` pointing to the Runtipi root directory (containing `app-data/` and `user-config/`). The container automatically mounts:

- **`/config`** ← `${ROOT_FOLDER_HOST}/app-data/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy/data/config`
- **`/compose`** ← `${ROOT_FOLDER_HOST}/user-config/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy`
- **`/var/run/docker.sock`** ← Host Docker socket

| Variable | Required | Default | Description |
| :--- | :---: | :---: | :--- |
| `PROXY_STORE_ID` | Yes | `migrated` | Store subfolder where `email-oauth2-proxy` is installed (`migrated` for migrated apps or `runtipi` for a fresh official store install). |
| `PROXY_HOST` | Yes | `email-oauth2-proxy` | Proxy container hostname on `tipi_main_network`. |
| `PROXY_CONTAINER` | No | `email-oauth2-proxy` | Exact Docker container name for restarts and `SIGUSR1`. |
| `ADMIN_USER` | No | `admin` | Web UI login username (when `ADMIN_PASSWORD` is set). |
| `ADMIN_PASSWORD` | No | *(empty)* | Web UI login password (leave blank to disable login). |
