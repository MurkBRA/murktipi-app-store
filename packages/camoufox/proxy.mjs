import crypto from "node:crypto";
import http from "node:http";
import net from "node:net";

const HOST = process.env.PROXY_BIND_HOST || "0.0.0.0";
const PORT = Number(process.env.PORT || "9223");
const INTERNAL_HOST = "127.0.0.1";
const INTERNAL_PORT = Number(process.env.INTERNAL_WS_PORT || "9224");
const INTERNAL_WS_PATH = ensureLeadingSlash(process.env.INTERNAL_WS_PATH || "/camoufox");
const WS_TOKEN = (process.env.WS_TOKEN || "").trim();
const HEADLESS_MODE = (process.env.HEADLESS || "virtual").trim();
const GEOIP_MODE = (process.env.GEOIP || "false").trim();
const CAMOUFOX_VERSION = (process.env.CAMOUFOX_LIB_VERSION || "0.5.9").trim();
const STARTED_AT = Date.now();

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

function isAuthorized(req) {
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

function probeUpstream(timeoutMs = 1500) {
  return new Promise((resolve) => {
    const socket = net.connect({ host: INTERNAL_HOST, port: INTERNAL_PORT });
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

function renderStatusHtml(req, ready) {
  const hostHeader = escapeHtml(req.headers.host || `localhost:${PORT}`);
  const publicPath = WS_TOKEN ? "/<WS_TOKEN>" : INTERNAL_WS_PATH;
  const wsExample = `ws://${hostHeader}${WS_TOKEN ? "/<YOUR_WS_TOKEN>" : "/"}`;
  const badgeColor = ready ? "#10b981" : "#f59e0b";
  const badgeLabel = ready ? "Ready" : "Starting";
  const authLabel = WS_TOKEN ? "Token Required (WS_TOKEN)" : "Open (No WS_TOKEN)";
  const uptimeSec = Math.floor((Date.now() - STARTED_AT) / 1000);

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Camoufox Playwright Server</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; font-family: system-ui, -apple-system, sans-serif; background: #0b1120; color: #e2e8f0; padding: 2rem; }
    .wrap { max-width: 780px; margin: 0 auto; }
    .card { background: #111827; border: 1px solid #1e293b; border-radius: 14px; padding: 1.5rem; margin-bottom: 1.25rem; }
    .row { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 1rem; }
    .badge { display: inline-flex; align-items: center; gap: .5rem; padding: .35rem .8rem; border-radius: 999px; font-weight: 600; font-size: .875rem; background: ${badgeColor}22; color: ${badgeColor}; border: 1px solid ${badgeColor}55; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: .75rem; margin-top: 1rem; }
    .stat { background: #0f172a; border: 1px solid #1e293b; border-radius: 10px; padding: .85rem; }
    .label { font-size: .75rem; color: #94a3b8; text-transform: uppercase; letter-spacing: .04em; }
    .val { font-size: 1rem; font-weight: 600; margin-top: .25rem; word-break: break-all; }
    pre { background: #020617; border: 1px solid #1e293b; border-radius: 10px; padding: 1rem; overflow-x: auto; font-size: .85rem; line-height: 1.45; }
    h1 { margin: 0; font-size: 1.5rem; }
    h2 { margin: 0 0 .75rem; font-size: 1.05rem; color: #f8fafc; }
    p { color: #94a3b8; margin: .4rem 0 0; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="row">
        <div>
          <h1>Camoufox Playwright Server</h1>
          <p>Anti-detect Firefox WebSocket server for Playwright, Node-RED, and n8n</p>
        </div>
        <span class="badge">${badgeLabel}</span>
      </div>
      <div class="grid">
        <div class="stat"><div class="label">Package Version</div><div class="val">v${escapeHtml(CAMOUFOX_VERSION)}</div></div>
        <div class="stat"><div class="label">Headless Mode</div><div class="val">${escapeHtml(HEADLESS_MODE)}</div></div>
        <div class="stat"><div class="label">GeoIP</div><div class="val">${escapeHtml(GEOIP_MODE || "false")}</div></div>
        <div class="stat"><div class="label">Active WS Clients</div><div class="val">${activeConnections}</div></div>
        <div class="stat"><div class="label">Authentication</div><div class="val">${escapeHtml(authLabel)}</div></div>
        <div class="stat"><div class="label">Uptime</div><div class="val">${uptimeSec}s</div></div>
      </div>
    </div>
    <div class="card">
      <h2>Playwright / Node-RED Connection Example</h2>
      <p>WebSocket path: <code>${escapeHtml(publicPath)}</code></p>
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

const server = http.createServer(async (req, res) => {
  const url = parseReqUrl(req.url);

  if (url.pathname === "/health") {
    const ready = await probeUpstream();
    sendJson(res, ready ? 200 : 503, {
      status: ready ? "ok" : "starting",
      ready,
      engine: "camoufox",
      version: CAMOUFOX_VERSION,
      headless: HEADLESS_MODE,
      activeConnections,
    });
    return;
  }

  if (url.pathname === "/json" || url.pathname === "/json/version") {
    if (!isAuthorized(req)) {
      sendJson(res, 401, { error: "Unauthorized" });
      return;
    }
    const ready = await probeUpstream();
    const hostHeader = req.headers.host || `localhost:${PORT}`;
    const publicPath = WS_TOKEN ? `/${WS_TOKEN}` : INTERNAL_WS_PATH;
    sendJson(res, ready ? 200 : 503, {
      ready,
      engine: "camoufox",
      browser: "firefox",
      version: CAMOUFOX_VERSION,
      wsEndpointPath: publicPath,
      wsEndpoint: `ws://${hostHeader}${publicPath}`,
    });
    return;
  }

  if (url.pathname === "/") {
    const ready = await probeUpstream();
    const html = renderStatusHtml(req, ready);
    res.writeHead(200, {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-store",
      "Content-Length": Buffer.byteLength(html),
    });
    res.end(html);
    return;
  }

  sendJson(res, 404, { error: "Not Found" });
});

server.on("upgrade", (req, clientSocket, head) => {
  if (!isAuthorized(req)) {
    clientSocket.write(
      "HTTP/1.1 401 Unauthorized\r\nConnection: close\r\nContent-Type: text/plain\r\nContent-Length: 12\r\n\r\nUnauthorized"
    );
    clientSocket.destroy();
    return;
  }

  const target = buildInternalTarget(req.url);
  const rawHeaders = [];
  for (let i = 0; i < req.rawHeaders.length; i += 2) {
    rawHeaders.push(`${req.rawHeaders[i]}: ${req.rawHeaders[i + 1]}`);
  }
  const handshake = `${req.method || "GET"} ${target} HTTP/${req.httpVersion || "1.1"}\r\n${rawHeaders.join("\r\n")}\r\n\r\n`;

  const upstreamSocket = net.connect({ host: INTERNAL_HOST, port: INTERNAL_PORT });
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
        "HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Type: text/plain\r\nContent-Length: 27\r\n\r\nCamoufox server unavailable"
      );
    }
    cleanup();
  });

  clientSocket.once("error", cleanup);
  upstreamSocket.once("close", cleanup);
  clientSocket.once("close", cleanup);
});

for (const sig of ["SIGTERM", "SIGINT"]) {
  process.on(sig, () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 3000).unref();
  });
}

server.listen(PORT, HOST, () => {
  console.log(
    `[camoufox-proxy] Listening on http://${HOST}:${PORT} -> ws://${INTERNAL_HOST}:${INTERNAL_PORT}${INTERNAL_WS_PATH} (auth=${WS_TOKEN ? "enabled" : "disabled"})`
  );
});
