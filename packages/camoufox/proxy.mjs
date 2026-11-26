import crypto from "node:crypto";
import fs from "node:fs";
import http from "node:http";
import https from "node:https";
import net from "node:net";

const HOST = process.env.PROXY_BIND_HOST || "0.0.0.0";
const PORT = Number(process.env.PORT || "9223");
const INTERNAL_HOST = "127.0.0.1";
const INTERNAL_PORT = Number(process.env.INTERNAL_WS_PORT || "9224");
const INTERNAL_WS_PATH = ensureLeadingSlash(process.env.INTERNAL_WS_PATH || "/camoufox");
const UI_MUX_PORT = Number(process.env.UI_MUX_PORT || "0");
const SELKIES_HTTP_PORT = Number(process.env.SELKIES_HTTP_PORT || "3000");
const SELKIES_HTTPS_PORT = Number(process.env.SELKIES_HTTPS_PORT || "3001");
const BOOTSTRAP_STATUS_FILE = (process.env.BOOTSTRAP_STATUS_FILE || "").trim();
const TLS_CERT_PATH = (process.env.TLS_CERT_PATH || "").trim();
const TLS_KEY_PATH = (process.env.TLS_KEY_PATH || "").trim();
const VIEWER_ENABLED = !["0", "false", "no", "off"].includes(
  String(process.env.VIEWER_ACTIVE || "true").trim().toLowerCase()
);
const WS_TOKEN = (process.env.WS_TOKEN || "").trim();
const HEADLESS_MODE = (process.env.HEADLESS || "virtual").trim();
const GEOIP_MODE = (process.env.GEOIP || "false").trim();
const CAMOUFOX_VERSION = (process.env.CAMOUFOX_LIB_VERSION || "0.5.9").trim();
const STARTED_AT = Date.now();

let tlsOptions = null;
if (TLS_CERT_PATH && TLS_KEY_PATH && fs.existsSync(TLS_CERT_PATH) && fs.existsSync(TLS_KEY_PATH)) {
  try {
    tlsOptions = {
      cert: fs.readFileSync(TLS_CERT_PATH),
      key: fs.readFileSync(TLS_KEY_PATH),
    };
  } catch {
    tlsOptions = null;
  }
}
const TLS_ENABLED = Boolean(tlsOptions);

let activeConnections = 0;

function ensureLeadingSlash(value) {
  return value.startsWith("/") ? value : `/${value}`;
}

function sha256(value) {
  return crypto.createHash("sha256").update(value, "utf8").digest();
}

function tokenMatches(candidate) {
  if (!WS_TOKEN) return true;
  if (!candidate || typeof candidate !== "string") return false;
  return crypto.timingSafeEqual(sha256(candidate), sha256(WS_TOKEN));
}

function parseReqUrl(rawUrl) {
  return new URL(rawUrl || "/", "http://localhost");
}

function isRequestTls(req) {
  return Boolean(req.socket && req.socket.encrypted);
}

function isSecureTransport(req) {
  if (isRequestTls(req)) return true;
  const fwdProto = String(req.headers["x-forwarded-proto"] || "")
    .toLowerCase()
    .split(",")[0]
    .trim();
  if (fwdProto === "https") return true;
  return String(req.headers["x-forwarded-ssl"] || "").toLowerCase().trim() === "on";
}

function extractHostname(hostHeader) {
  const raw = String(hostHeader || "").trim();
  if (!raw) return "localhost";
  if (raw.startsWith("[")) {
    const end = raw.indexOf("]");
    return end >= 0 ? raw.slice(0, end + 1) : raw;
  }
  const idx = raw.lastIndexOf(":");
  return idx >= 0 ? raw.slice(0, idx) : raw;
}

function isWsAuthorized(req) {
  if (!WS_TOKEN) return true;

  const url = parseReqUrl(req.url);
  const queryToken = url.searchParams.get("token");
  if (tokenMatches(queryToken)) return true;

  const trimmedPath = url.pathname.replace(/^\/+|\/+$/g, "");
  if (trimmedPath && tokenMatches(trimmedPath)) return true;

  const authHeader = String(req.headers.authorization || "");
  if (authHeader.toLowerCase().startsWith("bearer ")) {
    const bearer = authHeader.slice(7).trim();
    if (tokenMatches(bearer)) return true;
  }

  return false;
}

function buildInternalTarget(rawUrl) {
  const url = parseReqUrl(rawUrl);
  url.searchParams.delete("token");
  const qs = url.searchParams.toString();
  return qs ? `${INTERNAL_WS_PATH}?${qs}` : INTERNAL_WS_PATH;
}

function probeTcp(port, timeoutMs = 1500) {
  if (!port || port <= 0) return Promise.resolve(false);
  return new Promise((resolve) => {
    const socket = net.connect({ host: INTERNAL_HOST, port });
    let settled = false;
    const finish = (ok) => {
      if (settled) return;
      settled = true;
      socket.destroy();
      resolve(ok);
    };
    socket.setTimeout(timeoutMs);
    socket.once("connect", () => finish(true));
    socket.once("timeout", () => finish(false));
    socket.once("error", () => finish(false));
  });
}

function escapeHtml(text) {
  return String(text)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function readBootstrapStatus() {
  if (!BOOTSTRAP_STATUS_FILE) return null;
  try {
    const raw = fs.readFileSync(BOOTSTRAP_STATUS_FILE, "utf8");
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" ? parsed : null;
  } catch {
    return null;
  }
}

function getSelkiesUiUrl(req) {
  const hostOnly = escapeHtml(extractHostname(req.headers.host || `localhost:${PORT}`));
  const uiPort = UI_MUX_PORT > 0 ? UI_MUX_PORT : SELKIES_HTTPS_PORT;
  const scheme = isSecureTransport(req) || UI_MUX_PORT > 0 ? "https" : "http";
  return `${scheme}://${hostOnly}:${uiPort}/`;
}

function renderStatusHtml(req, ready, viewerReady, bootstrap) {
  const rawHost = req.headers.host || `localhost:${PORT}`;
  const hostHeader = escapeHtml(rawHost);
  const publicPath = WS_TOKEN ? "/<WS_TOKEN>" : INTERNAL_WS_PATH;
  const wsScheme = isSecureTransport(req) ? "wss" : "ws";
  const wsExample = `${wsScheme}://${hostHeader}${WS_TOKEN ? "/<YOUR_WS_TOKEN>" : "/"}`;
  const downloading = !ready && bootstrap?.phase === "downloading";
  const badgeColor = ready ? "#10b981" : "#f59e0b";
  const badgeLabel = ready ? "Playwright Ready" : downloading ? "Downloading Browser" : "Starting";
  const viewerBadgeColor = !VIEWER_ENABLED ? "#64748b" : viewerReady ? "#38bdf8" : "#f59e0b";
  const viewerBadgeLabel = !VIEWER_ENABLED
    ? "Viewer Off (Native Headless)"
    : viewerReady
      ? "Selkies Ready"
      : "Selkies Starting";
  const authLabel = WS_TOKEN ? "Token Required (WS_TOKEN)" : "Open (No WS_TOKEN)";
  const uptimeSec = Math.floor((Date.now() - STARTED_AT) / 1000);
  const selkiesUrl = getSelkiesUiUrl(req);
  const bootstrapBanner =
    !ready && bootstrap?.message
      ? `<p class="muted" style="color:#fbbf24;margin-top:.75rem;">${escapeHtml(bootstrap.message)}</p>`
      : "";
  const autoRefreshMeta = !ready ? '<meta http-equiv="refresh" content="5" />' : "";
  const viewerSection = VIEWER_ENABLED
    ? `<div class="actions">
         <a class="btn" href="${selkiesUrl}" target="_blank" rel="noopener">Open Live Browser Viewer (Selkies)</a>
       </div>`
    : `<p class="muted">Live browser viewer requires <code>HEADLESS=virtual</code> (currently running in native headless mode).</p>`;

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  ${autoRefreshMeta}
  <title>Camoufox Playwright Server</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; font-family: system-ui, -apple-system, sans-serif; background: #0b1120; color: #e2e8f0; padding: 1.5rem; }
    .wrap { max-width: 960px; margin: 0 auto; }
    .card { background: #111827; border: 1px solid #1e293b; border-radius: 14px; padding: 1.4rem; margin-bottom: 1.15rem; }
    .row { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 1rem; }
    .badges { display: flex; flex-wrap: wrap; gap: .5rem; }
    .badge { display: inline-flex; align-items: center; gap: .45rem; padding: .35rem .8rem; border-radius: 999px; font-weight: 600; font-size: .85rem; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: .75rem; margin-top: 1rem; }
    .stat { background: #0f172a; border: 1px solid #1e293b; border-radius: 10px; padding: .8rem; }
    .label { font-size: .72rem; color: #94a3b8; text-transform: uppercase; letter-spacing: .04em; }
    .val { font-size: .95rem; font-weight: 600; margin-top: .25rem; word-break: break-all; }
    .actions { display: flex; flex-wrap: wrap; gap: .75rem; margin-top: 1rem; }
    .btn { display: inline-flex; align-items: center; justify-content: center; background: #f97316; color: #0b1120; text-decoration: none; border: none; border-radius: 8px; padding: .6rem 1rem; font-weight: 700; font-size: .9rem; cursor: pointer; }
    .btn:hover { background: #fb923c; }
    pre { background: #020617; border: 1px solid #1e293b; border-radius: 10px; padding: 1rem; overflow-x: auto; font-size: .85rem; line-height: 1.45; }
    h1 { margin: 0; font-size: 1.45rem; }
    h2 { margin: 0 0 .6rem; font-size: 1.05rem; color: #f8fafc; }
    p, .muted { color: #94a3b8; margin: .4rem 0 0; font-size: .92rem; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="row">
        <div>
          <h1>Camoufox Playwright + Selkies</h1>
          <p>Anti-detect Firefox WebSocket server paired with LSIO Selkies HTML5 streaming</p>
        </div>
        <div class="badges">
          <span class="badge" style="background:${badgeColor}22;color:${badgeColor};border:1px solid ${badgeColor}55;">${badgeLabel}</span>
          <span class="badge" style="background:${viewerBadgeColor}22;color:${viewerBadgeColor};border:1px solid ${viewerBadgeColor}55;">${viewerBadgeLabel}</span>
        </div>
      </div>
      <div class="grid">
        <div class="stat"><div class="label">Camoufox</div><div class="val">v${escapeHtml(CAMOUFOX_VERSION)}</div></div>
        <div class="stat"><div class="label">Headless Mode</div><div class="val">${escapeHtml(HEADLESS_MODE)}</div></div>
        <div class="stat"><div class="label">GeoIP</div><div class="val">${escapeHtml(GEOIP_MODE || "false")}</div></div>
        <div class="stat"><div class="label">Active WS Clients</div><div class="val">${activeConnections}</div></div>
        <div class="stat"><div class="label">Authentication</div><div class="val">${escapeHtml(authLabel)}</div></div>
        <div class="stat"><div class="label">Uptime</div><div class="val">${uptimeSec}s</div></div>
      </div>
      ${bootstrapBanner}
      ${viewerSection}
    </div>
    <div class="card">
      <h2>Playwright / Node-RED Connection</h2>
      <p>WebSocket path: <code>${escapeHtml(publicPath)}</code> (live browser opens inside Selkies whenever a script calls <code>newPage()</code>)</p>
      <pre>const browser = await playwright.firefox.connect('${wsExample}');
const page = await browser.newPage();
await page.goto('https://example.com');
await browser.close();</pre>
    </div>
    <div class="card">
      <h2>Python Playwright Example</h2>
      <pre>async with async_playwright() as p:
    browser = await p.firefox.connect("${wsExample}")
    page = await browser.new_page()
    await page.goto("https://example.com")
    await browser.close()</pre>
    </div>
  </div>
</body>
</html>`;
}

function sendJson(res, status, payload) {
  const body = JSON.stringify(payload);
  res.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Cache-Control": "no-store",
    "Content-Length": Buffer.byteLength(body),
  });
  res.end(body);
}

const handleHttpRequest = async (req, res) => {
  const url = parseReqUrl(req.url);

  if (url.pathname === "/health") {
    const ready = await probeTcp(INTERNAL_PORT);
    const viewerReady = VIEWER_ENABLED ? await probeTcp(SELKIES_HTTP_PORT) : true;
    const healthy = ready && (!VIEWER_ENABLED || viewerReady);
    const bootstrap = readBootstrapStatus();
    sendJson(res, healthy ? 200 : 503, {
      status: healthy ? "ok" : (bootstrap?.phase || "starting"),
      ready,
      viewerEnabled: VIEWER_ENABLED,
      viewerReady,
      tlsEnabled: TLS_ENABLED,
      uiMuxPort: UI_MUX_PORT || null,
      phase: healthy ? "ready" : (bootstrap?.phase || "starting"),
      message: healthy ? "Ready" : (bootstrap?.message || "Starting..."),
      engine: "camoufox",
      version: CAMOUFOX_VERSION,
      headless: HEADLESS_MODE,
      activeConnections,
    });
    return;
  }

  if (url.pathname === "/json" || url.pathname === "/json/version") {
    if (!isWsAuthorized(req)) {
      sendJson(res, 401, { error: "Unauthorized" });
      return;
    }
    const ready = await probeTcp(INTERNAL_PORT);
    const hostHeader = req.headers.host || `localhost:${PORT}`;
    const publicPath = WS_TOKEN ? `/${WS_TOKEN}` : INTERNAL_WS_PATH;
    const wsScheme = isSecureTransport(req) ? "wss" : "ws";
    sendJson(res, ready ? 200 : 503, {
      ready,
      viewerEnabled: VIEWER_ENABLED,
      viewerUrl: VIEWER_ENABLED ? getSelkiesUiUrl(req) : null,
      tlsEnabled: TLS_ENABLED,
      engine: "camoufox",
      browser: "firefox",
      version: CAMOUFOX_VERSION,
      wsEndpointPath: publicPath,
      wsEndpoint: `${wsScheme}://${hostHeader}${publicPath}`,
    });
    return;
  }

  if (url.pathname === "/viewer" || url.pathname.startsWith("/viewer/")) {
    res.writeHead(302, {
      Location: getSelkiesUiUrl(req),
      "Cache-Control": "no-store",
    });
    res.end();
    return;
  }

  if (url.pathname === "/") {
    const [ready, viewerReady] = await Promise.all([
      probeTcp(INTERNAL_PORT),
      VIEWER_ENABLED ? probeTcp(SELKIES_HTTP_PORT) : Promise.resolve(false),
    ]);
    const html = renderStatusHtml(req, ready, viewerReady, readBootstrapStatus());
    res.writeHead(200, {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-store",
      "Content-Length": Buffer.byteLength(html),
    });
    res.end(html);
    return;
  }

  sendJson(res, 404, { error: "Not Found" });
};

function pipeRawUpgrade(req, clientSocket, head, targetPort, targetPath) {
  const rawHeaders = [];
  for (let i = 0; i < req.rawHeaders.length; i += 2) {
    rawHeaders.push(`${req.rawHeaders[i]}: ${req.rawHeaders[i + 1]}`);
  }
  const handshake = `${req.method || "GET"} ${targetPath} HTTP/${req.httpVersion || "1.1"}\r\n${rawHeaders.join("\r\n")}\r\n\r\n`;

  const upstreamSocket = net.connect({ host: INTERNAL_HOST, port: targetPort });
  let cleaned = false;
  let counted = false;

  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    if (counted) {
      counted = false;
      activeConnections = Math.max(0, activeConnections - 1);
    }
    clientSocket.destroy();
    upstreamSocket.destroy();
  };

  upstreamSocket.once("connect", () => {
    counted = true;
    activeConnections += 1;
    upstreamSocket.write(handshake);
    if (head && head.length > 0) {
      upstreamSocket.write(head);
    }
    upstreamSocket.pipe(clientSocket);
    clientSocket.pipe(upstreamSocket);
  });

  upstreamSocket.once("error", () => {
    if (!counted && !cleaned) {
      clientSocket.write(
        "HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Type: text/plain\r\nContent-Length: 19\r\n\r\nService unavailable"
      );
    }
    cleanup();
  });

  clientSocket.once("error", cleanup);
  upstreamSocket.once("close", cleanup);
  clientSocket.once("close", cleanup);
}

const handleUpgrade = (req, clientSocket, head) => {
  if (!isWsAuthorized(req)) {
    clientSocket.write(
      "HTTP/1.1 401 Unauthorized\r\nConnection: close\r\nContent-Type: text/plain\r\nContent-Length: 12\r\n\r\nUnauthorized"
    );
    clientSocket.destroy();
    return;
  }

  pipeRawUpgrade(req, clientSocket, head, INTERNAL_PORT, buildInternalTarget(req.url));
};

const httpServer = http.createServer(handleHttpRequest);
httpServer.on("upgrade", handleUpgrade);

const httpsServer = tlsOptions ? https.createServer(tlsOptions, handleHttpRequest) : null;
if (httpsServer) {
  httpsServer.on("upgrade", handleUpgrade);
}

const server = tlsOptions
  ? net.createServer((socket) => {
      socket.on("error", () => {
        if (!socket.destroyed) socket.destroy();
      });
      socket.setTimeout(10000, () => {
        if (!socket.destroyed) socket.destroy();
      });
      const onReadable = () => {
        const firstByte = socket.read(1);
        if (firstByte === null) {
          if (socket.readableEnded) {
            socket.destroy();
          } else {
            socket.once("readable", onReadable);
          }
          return;
        }
        socket.setTimeout(0);
        socket.unshift(firstByte);
        if (firstByte[0] === 0x16) {
          httpsServer.emit("connection", socket);
        } else {
          httpServer.emit("connection", socket);
        }
      };
      socket.once("readable", onReadable);
    })
  : httpServer;

// Wire-speed HTTP/HTTPS port multiplexer for LSIO Nginx Selkies UI (3000 HTTP / 3001 HTTPS).
let uiMuxServer = null;
if (UI_MUX_PORT > 0) {
  uiMuxServer = net.createServer((clientSocket) => {
    clientSocket.on("error", () => {
      if (!clientSocket.destroyed) clientSocket.destroy();
    });
    clientSocket.setTimeout(10000, () => {
      if (!clientSocket.destroyed) clientSocket.destroy();
    });
    const onReadable = () => {
      const firstByte = clientSocket.read(1);
      if (firstByte === null) {
        if (clientSocket.readableEnded) {
          clientSocket.destroy();
        } else {
          clientSocket.once("readable", onReadable);
        }
        return;
      }
      clientSocket.setTimeout(0);
      const targetPort = firstByte[0] === 0x16 ? SELKIES_HTTPS_PORT : SELKIES_HTTP_PORT;
      const upstreamSocket = net.connect({ host: INTERNAL_HOST, port: targetPort });
      let closed = false;
      const cleanup = () => {
        if (closed) return;
        closed = true;
        clientSocket.destroy();
        upstreamSocket.destroy();
      };
      upstreamSocket.once("connect", () => {
        upstreamSocket.write(firstByte);
        upstreamSocket.pipe(clientSocket);
        clientSocket.pipe(upstreamSocket);
      });
      upstreamSocket.once("error", cleanup);
      clientSocket.once("error", cleanup);
      upstreamSocket.once("close", cleanup);
      clientSocket.once("close", cleanup);
    };
    clientSocket.once("readable", onReadable);
  });
}

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => {
    uiMuxServer?.close();
    server.close(() => process.exit(0));
    httpServer.closeAllConnections?.();
    httpsServer?.closeAllConnections?.();
    setTimeout(() => process.exit(0), 3000).unref();
  });
}

server.listen(PORT, HOST, () => {
  const protoDesc = TLS_ENABLED ? `http+https://${HOST}:${PORT}` : `http://${HOST}:${PORT}`;
  console.log(
    `[camoufox-proxy] Listening on ${protoDesc} -> ws://${INTERNAL_HOST}:${INTERNAL_PORT}${INTERNAL_WS_PATH} (tls=${TLS_ENABLED ? "enabled" : "disabled"}, auth=${WS_TOKEN ? "enabled" : "disabled"})`
  );
});

if (uiMuxServer) {
  uiMuxServer.listen(UI_MUX_PORT, HOST, () => {
    console.log(
      `[camoufox-ui-mux] Listening on http+https://${HOST}:${UI_MUX_PORT} -> :${SELKIES_HTTP_PORT} (HTTP) / :${SELKIES_HTTPS_PORT} (HTTPS)`
    );
  });
}
