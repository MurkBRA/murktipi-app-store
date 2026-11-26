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
const SELKIES_PORT = Number(process.env.SELKIES_PORT || "8080");
const BOOTSTRAP_STATUS_FILE = (process.env.BOOTSTRAP_STATUS_FILE || "").trim();
const TLS_CERT_PATH = (process.env.TLS_CERT_PATH || "").trim();
const TLS_KEY_PATH = (process.env.TLS_KEY_PATH || "").trim();
const VIEWER_PREFIX = "/viewer";
const VIEWER_ENABLED = !["0", "false", "no", "off"].includes(
  String(process.env.VIEWER_ACTIVE || "true").trim().toLowerCase()
);
const WS_TOKEN = (process.env.WS_TOKEN || "").trim();
const HEADLESS_MODE = (process.env.HEADLESS || "virtual").trim();
const GEOIP_MODE = (process.env.GEOIP || "false").trim();
const CAMOUFOX_VERSION = (process.env.CAMOUFOX_LIB_VERSION || "0.5.9").trim();
const STARTED_AT = Date.now();
const VIEWER_COOKIE_NAME = "camoufox_viewer_auth";

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
    return end >= 0 ? raw.slice(1, end).toLowerCase() : raw.toLowerCase();
  }
  const idx = raw.lastIndexOf(":");
  return (idx >= 0 ? raw.slice(0, idx) : raw).toLowerCase();
}

function isLocalHostHeader(hostHeader) {
  const host = extractHostname(hostHeader);
  return host === "localhost" || host === "127.0.0.1" || host === "::1";
}

function ensureLeadingSlash(value) {
  return value.startsWith("/") ? value : `/${value}`;
}

function sha256(value) {
  return crypto.createHash("sha256").update(value, "utf8").digest();
}

function tokenDigestHex(value) {
  return crypto.createHash("sha256").update(`viewer:${value}`, "utf8").digest("hex");
}

function tokenMatches(candidate) {
  if (!WS_TOKEN) return true;
  if (!candidate || typeof candidate !== "string") return false;
  return crypto.timingSafeEqual(sha256(candidate), sha256(WS_TOKEN));
}

function parseCookies(cookieHeader) {
  const cookies = {};
  if (!cookieHeader) return cookies;
  for (const part of String(cookieHeader).split(";")) {
    const eq = part.indexOf("=");
    if (eq === -1) continue;
    const key = part.slice(0, eq).trim();
    const val = part.slice(eq + 1).trim();
    if (key) cookies[key] = decodeURIComponent(val);
  }
  return cookies;
}

function parseReqUrl(rawUrl) {
  return new URL(rawUrl || "/", "http://localhost");
}

function checkViewerAuth(req) {
  if (!WS_TOKEN) return { ok: true, setCookie: false };

  const url = parseReqUrl(req.url);
  const queryToken = url.searchParams.get("token");
  if (tokenMatches(queryToken)) {
    return { ok: true, setCookie: true };
  }

  const cookies = parseCookies(req.headers.cookie);
  const expectedCookie = tokenDigestHex(WS_TOKEN);
  const cookieVal = cookies[VIEWER_COOKIE_NAME];
  if (
    typeof cookieVal === "string" &&
    cookieVal.length === expectedCookie.length &&
    crypto.timingSafeEqual(Buffer.from(cookieVal), Buffer.from(expectedCookie))
  ) {
    return { ok: true, setCookie: false };
  }

  const authHeader = String(req.headers.authorization || "");
  if (authHeader.toLowerCase().startsWith("bearer ")) {
    if (tokenMatches(authHeader.slice(7).trim())) {
      return { ok: true, setCookie: false };
    }
  } else if (authHeader.toLowerCase().startsWith("basic ")) {
    try {
      const decoded = Buffer.from(authHeader.slice(6).trim(), "base64").toString("utf8");
      const colon = decoded.indexOf(":");
      const password = colon >= 0 ? decoded.slice(colon + 1) : decoded;
      if (tokenMatches(password)) {
        return { ok: true, setCookie: false };
      }
    } catch {
      // Ignore malformed Basic Auth header
    }
  }

  return { ok: false, setCookie: false };
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

function isViewerPath(pathname) {
  return pathname === VIEWER_PREFIX || pathname.startsWith(`${VIEWER_PREFIX}/`);
}

function buildInternalTarget(rawUrl) {
  const url = parseReqUrl(rawUrl);
  url.searchParams.delete("token");
  const qs = url.searchParams.toString();
  return qs ? `${INTERNAL_WS_PATH}?${qs}` : INTERNAL_WS_PATH;
}

function probeTcp(port, timeoutMs = 1500) {
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

function renderStatusHtml(req, ready, viewerReady, bootstrap) {
  const rawHost = req.headers.host || `localhost:${PORT}`;
  const hostHeader = escapeHtml(rawHost);
  const secure = isSecureTransport(req);
  const needsHttpsForViewer = TLS_ENABLED && !secure && !isLocalHostHeader(rawHost);
  const viewerHref = needsHttpsForViewer ? `https://${hostHeader}/viewer/` : "/viewer/";
  const httpsDashboardHref = `https://${hostHeader}/?embed=1`;
  const publicPath = WS_TOKEN ? "/<WS_TOKEN>" : INTERNAL_WS_PATH;
  const wsScheme = secure ? "wss" : "ws";
  const wsExample = `${wsScheme}://${hostHeader}${WS_TOKEN ? "/<YOUR_WS_TOKEN>" : "/"}`;
  const downloading = !ready && bootstrap?.phase === "downloading";
  const badgeColor = ready ? "#10b981" : "#f59e0b";
  const badgeLabel = ready ? "Playwright Ready" : downloading ? "Downloading Browser" : "Starting";
  const bootstrapBanner =
    !ready && bootstrap?.message
      ? `<p class="muted" style="color:#fbbf24;margin-top:.75rem;">${escapeHtml(bootstrap.message)}</p>`
      : "";
  const autoRefreshMeta = !ready ? '<meta http-equiv="refresh" content="5" />' : "";
  const viewerBadgeColor = !VIEWER_ENABLED ? "#64748b" : viewerReady ? "#38bdf8" : "#f59e0b";
  const viewerBadgeLabel = !VIEWER_ENABLED
    ? "Viewer Off (Native Headless)"
    : viewerReady
      ? "Selkies Ready"
      : "Selkies Starting";
  const authLabel = WS_TOKEN ? "Token Required (WS_TOKEN)" : "Open (No WS_TOKEN)";
  const uptimeSec = Math.floor((Date.now() - STARTED_AT) / 1000);
  const httpsHint = needsHttpsForViewer
    ? `<p class="muted">Selkies WebCodecs requires HTTPS on non-localhost hosts. Opening the viewer upgrades to <code>https://${hostHeader}/viewer/</code> (accept the self-signed certificate once if accessing directly by LAN IP).</p>`
    : "";
  const viewerSection = VIEWER_ENABLED
    ? `<div class="actions">
         <a class="btn" href="${viewerHref}" target="_blank" rel="noopener">Open Live Browser Viewer (Selkies)</a>
         <button class="btn btn-secondary" type="button" onclick="toggleEmbed()">Toggle Embedded View</button>
       </div>
       ${httpsHint}
       <div id="embedWrap" class="embed-wrap" style="display:none;">
         ${
           needsHttpsForViewer
             ? `<p class="muted">Embedded viewer requires HTTPS on this host. <a href="${httpsDashboardHref}" style="color:#fb923c;">Switch dashboard to HTTPS</a> or open the viewer in a new tab above.</p>`
             : `<iframe id="viewerFrame" title="Selkies Live Browser" style="width:100%;height:640px;border:1px solid #1e293b;border-radius:10px;background:#020617;"></iframe>`
         }
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
    .btn-secondary { background: #1e293b; color: #e2e8f0; }
    .btn-secondary:hover { background: #334155; }
    .embed-wrap { margin-top: 1rem; }
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
          <p>Anti-detect Firefox WebSocket server with live Selkies HTML5 display streaming</p>
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
      <p>WebSocket path: <code>${escapeHtml(publicPath)}</code> (live browser opens in Selkies whenever a script calls <code>newPage()</code>)</p>
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
  <script>
    function toggleEmbed() {
      const wrap = document.getElementById('embedWrap');
      const frame = document.getElementById('viewerFrame');
      if (!wrap) return;
      const open = wrap.style.display === 'none';
      wrap.style.display = open ? 'block' : 'none';
      if (open && frame && !frame.getAttribute('src')) {
        frame.setAttribute('src', '/viewer/');
      }
    }
    if (new URLSearchParams(window.location.search).get('embed') === '1') {
      toggleEmbed();
    }
  </script>
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

function proxyViewerHttp(req, res) {
  const headers = { ...req.headers };
  delete headers.authorization;
  if (isRequestTls(req) && !headers["x-forwarded-proto"]) {
    headers["x-forwarded-proto"] = "https";
  }
  const upstreamReq = http.request(
    {
      hostname: INTERNAL_HOST,
      port: SELKIES_PORT,
      path: req.url,
      method: req.method,
      headers,
    },
    (upstreamRes) => {
      res.writeHead(upstreamRes.statusCode || 502, upstreamRes.headers);
      upstreamRes.pipe(res);
    }
  );
  upstreamReq.on("error", () => {
    if (!res.headersSent) {
      sendJson(res, 503, { error: "Selkies live viewer is starting or unavailable." });
    } else {
      res.destroy();
    }
  });
  req.pipe(upstreamReq);
}

const handleHttpRequest = async (req, res) => {
  const url = parseReqUrl(req.url);

  if (url.pathname === "/health") {
    const ready = await probeTcp(INTERNAL_PORT);
    const viewerReady = VIEWER_ENABLED ? await probeTcp(SELKIES_PORT) : true;
    const healthy = ready && (!VIEWER_ENABLED || viewerReady);
    const bootstrap = readBootstrapStatus();
    sendJson(res, healthy ? 200 : 503, {
      status: healthy ? "ok" : (bootstrap?.phase || "starting"),
      ready,
      viewerEnabled: VIEWER_ENABLED,
      viewerReady,
      tlsEnabled: TLS_ENABLED,
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
      viewerPath: VIEWER_ENABLED ? `${VIEWER_PREFIX}/` : null,
      tlsEnabled: TLS_ENABLED,
      engine: "camoufox",
      browser: "firefox",
      version: CAMOUFOX_VERSION,
      wsEndpointPath: publicPath,
      wsEndpoint: `${wsScheme}://${hostHeader}${publicPath}`,
    });
    return;
  }

  if (isViewerPath(url.pathname)) {
    if (!VIEWER_ENABLED) {
      sendJson(res, 503, { error: "Live viewer is disabled when HEADLESS=true." });
      return;
    }
    if (TLS_ENABLED && !isSecureTransport(req) && !isLocalHostHeader(req.headers.host)) {
      const hostHeader = req.headers.host || `localhost:${PORT}`;
      const targetUrl = `https://${hostHeader}${req.url || `${VIEWER_PREFIX}/`}`;
      res.writeHead(302, {
        Location: targetUrl,
        "Cache-Control": "no-store",
      });
      res.end();
      return;
    }
    const auth = checkViewerAuth(req);
    if (!auth.ok) {
      res.writeHead(401, {
        "WWW-Authenticate": 'Basic realm="Camoufox Live Viewer"',
        "Content-Type": "text/plain; charset=utf-8",
        "Cache-Control": "no-store",
      });
      res.end("Unauthorized");
      return;
    }
    if (auth.setCookie) {
      const cleanUrl = new URL(url.toString());
      cleanUrl.searchParams.delete("token");
      const targetPath = cleanUrl.pathname === VIEWER_PREFIX ? `${VIEWER_PREFIX}/` : cleanUrl.pathname;
      const qs = cleanUrl.searchParams.toString();
      const secureAttr = isSecureTransport(req) ? "; Secure" : "";
      res.writeHead(302, {
        "Set-Cookie": `${VIEWER_COOKIE_NAME}=${tokenDigestHex(WS_TOKEN)}; Path=${VIEWER_PREFIX}; HttpOnly; SameSite=Lax${secureAttr}`,
        Location: qs ? `${targetPath}?${qs}` : targetPath,
      });
      res.end();
      return;
    }
    if (url.pathname === VIEWER_PREFIX) {
      const qs = url.searchParams.toString();
      res.writeHead(302, {
        Location: qs ? `${VIEWER_PREFIX}/?${qs}` : `${VIEWER_PREFIX}/`,
      });
      res.end();
      return;
    }
    proxyViewerHttp(req, res);
    return;
  }

  if (url.pathname === "/") {
    const [ready, viewerReady] = await Promise.all([
      probeTcp(INTERNAL_PORT),
      VIEWER_ENABLED ? probeTcp(SELKIES_PORT) : Promise.resolve(false),
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

function pipeRawUpgrade(req, clientSocket, head, targetPort, targetPath, stripAuthHeader = false) {
  const rawHeaders = [];
  let hasFwdProto = false;
  for (let i = 0; i < req.rawHeaders.length; i += 2) {
    const name = req.rawHeaders[i];
    const lower = name.toLowerCase();
    if (stripAuthHeader && lower === "authorization") continue;
    if (lower === "x-forwarded-proto") hasFwdProto = true;
    rawHeaders.push(`${name}: ${req.rawHeaders[i + 1]}`);
  }
  if (isRequestTls(req) && !hasFwdProto) {
    rawHeaders.push("X-Forwarded-Proto: https");
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
  const url = parseReqUrl(req.url);

  if (isViewerPath(url.pathname)) {
    if (!VIEWER_ENABLED) {
      clientSocket.write(
        "HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 15\r\n\r\nViewer disabled"
      );
      clientSocket.destroy();
      return;
    }
    if (!checkViewerAuth(req).ok) {
      clientSocket.write(
        "HTTP/1.1 401 Unauthorized\r\nConnection: close\r\nContent-Length: 12\r\n\r\nUnauthorized"
      );
      clientSocket.destroy();
      return;
    }
    pipeRawUpgrade(req, clientSocket, head, SELKIES_PORT, req.url || `${VIEWER_PREFIX}/`, true);
    return;
  }

  if (!isWsAuthorized(req)) {
    clientSocket.write(
      "HTTP/1.1 401 Unauthorized\r\nConnection: close\r\nContent-Type: text/plain\r\nContent-Length: 12\r\n\r\nUnauthorized"
    );
    clientSocket.destroy();
    return;
  }

  pipeRawUpgrade(req, clientSocket, head, INTERNAL_PORT, buildInternalTarget(req.url), false);
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

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => {
    server.close(() => process.exit(0));
    httpServer.closeAllConnections?.();
    httpsServer?.closeAllConnections?.();
    setTimeout(() => process.exit(0), 3000).unref();
  });
}

server.listen(PORT, HOST, () => {
  const protoDesc = TLS_ENABLED ? `http+https://${HOST}:${PORT}` : `http://${HOST}:${PORT}`;
  console.log(
    `[camoufox-proxy] Listening on ${protoDesc} -> ws://${INTERNAL_HOST}:${INTERNAL_PORT}${INTERNAL_WS_PATH} (viewer=${VIEWER_ENABLED ? VIEWER_PREFIX + "/ -> :" + SELKIES_PORT : "disabled"}, tls=${TLS_ENABLED ? "enabled" : "disabled"}, auth=${WS_TOKEN ? "enabled" : "disabled"})`
  );
});
