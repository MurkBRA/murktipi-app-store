import http.client
import http.server
import json
import os
import py_compile
import shutil
import socket
import ssl
import subprocess
import sys
import threading
import time
from PIL import Image


def test_camoufox_config_fields_and_secrets(get_app_config):
    config = get_app_config("camoufox")
    assert config["id"] == "camoufox"
    assert config["name"] == "Camoufox"
    assert config["port"] == 3003
    assert config.get("https") is True
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] == "0.5.9"
    assert config["version"] != "latest"
    assert config["tipi_version"] >= 5
    assert set(config["categories"]) == {"development", "automation", "utilities"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var

    expected_vars = {
        "CONTAINER_NAME",
        "PUID",
        "PGID",
        "CUSTOM_USER",
        "PASSWORD",
        "WS_TOKEN",
        "HEADLESS",
        "AUTOSTART_BROWSER",
        "SLIM_FONTS",
        "GEOIP",
        "OS_TARGET",
        "LOCALE",
        "BLOCK_IMAGES",
        "BLOCK_WEBRTC",
        "DISABLE_COOP",
        "PROXY_SERVER",
        "PROXY_USERNAME",
        "PROXY_PASSWORD",
        "SHM_SIZE",
    }
    assert set(fields_by_var.keys()) == expected_vars

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "camoufox"

    headless = fields_by_var["HEADLESS"]
    assert headless["required"] is True
    assert headless["default"] == "virtual"
    assert [o["value"] for o in headless["options"]] == ["virtual", "true"]

    for secret_var in ("PASSWORD", "WS_TOKEN", "PROXY_PASSWORD"):
        assert fields_by_var[secret_var]["type"] == "password"
        assert fields_by_var[secret_var]["required"] is False
        assert "default" not in fields_by_var[secret_var]

    for opt_var in expected_vars - {"CONTAINER_NAME", "HEADLESS"}:
        assert fields_by_var[opt_var]["required"] is False
        assert "default" not in fields_by_var[opt_var]


def test_camoufox_compose_structure_and_passthrough(get_app_config, get_app_compose):
    config = get_app_config("camoufox")
    compose = get_app_compose("camoufox")
    services = compose["services"]

    assert "camoufox" in services
    svc = services["camoufox"]
    assert svc.get("image") == f"ghcr.io/murkbra/camoufox:{config['version']}"
    assert svc.get("container_name") == "${CONTAINER_NAME:-camoufox}"
    assert svc.get("restart") == "unless-stopped"
    assert svc.get("shm_size") == "${SHM_SIZE:-1gb}"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:3003" in ports
    assert "9223:9223" in ports

    volumes = [str(v) for v in svc.get("volumes", [])]
    assert "${APP_DATA_DIR}/data/config:/config" in volumes

    env = svc.get("environment", [])
    assert isinstance(env, list)
    assert "HEADLESS=${HEADLESS:-virtual}" in env
    assert "TZ=${TZ}" in env
    for var in (
        "PUID",
        "PGID",
        "CUSTOM_USER",
        "PASSWORD",
        "AUTOSTART_BROWSER",
        "SLIM_FONTS",
        "WS_TOKEN",
        "GEOIP",
        "OS_TARGET",
        "LOCALE",
        "BLOCK_IMAGES",
        "BLOCK_WEBRTC",
        "DISABLE_COOP",
        "PROXY_SERVER",
        "PROXY_USERNAME",
        "PROXY_PASSWORD",
    ):
        assert var in env
        assert not any(str(e).startswith(f"{var}=") for e in env)

    assert "healthcheck" in svc
    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 3003


def test_camoufox_package_dockerfile_workflow_and_builder(repo_root, get_app_config):
    config = get_app_config("camoufox")
    pkg_dir = repo_root / "packages" / "camoufox"
    for filename in (
        "Dockerfile",
        "requirements.txt",
        "icon.png",
        "config_builder.py",
        "proxy.mjs",
        "server.py",
        "root/defaults/menu.xml",
        "root/defaults/autostart",
        "root/usr/bin/wrapped-camoufox",
        "root/etc/s6-overlay/s6-rc.d/svc-camoufox/type",
        "root/etc/s6-overlay/s6-rc.d/svc-camoufox/run",
        "root/etc/s6-overlay/s6-rc.d/user/contents.d/svc-camoufox",
    ):
        assert (pkg_dir / filename).exists(), f"Missing packages/camoufox/{filename}"

    req_text = (pkg_dir / "requirements.txt").read_text(encoding="utf-8")
    assert f"camoufox[geoip]=={config['version']}" in req_text
    assert "selkies==" in req_text

    df_text = (pkg_dir / "Dockerfile").read_text(encoding="utf-8")
    assert "FROM ghcr.io/linuxserver/baseimage-selkies:debiantrixie" in df_text
    assert "NO_DECOR=true" in df_text
    assert "TITLE=Camoufox" in df_text
    assert "ensure_fpgen_model" in df_text
    assert "camoufox fetch" not in df_text
    assert "EXPOSE 3000 3001 3003 9223" in df_text

    workflow = repo_root / ".github" / "workflows" / "docker-camoufox.yml"
    assert workflow.exists()
    wf_text = workflow.read_text(encoding="utf-8")
    assert "ghcr.io" in wf_text
    assert "murkbra/camoufox" in wf_text
    assert "linux/amd64,linux/arm64" in wf_text
    assert "steps.appver.outputs.version" in wf_text

    sys.path.insert(0, str(pkg_dir))
    try:
        from config_builder import build_camoufox_kwargs, parse_slim_fonts, parse_viewer_enabled, parse_viewer_resolution

        assert parse_viewer_enabled({}) is True
        assert parse_viewer_enabled({"HEADLESS": "true"}) is False
        assert parse_viewer_resolution({}) == (1920, 1080)
        assert parse_slim_fonts({"SLIM_FONTS": "true"}) is True

        built = build_camoufox_kwargs(
            {"OS_TARGET": "linux", "GEOIP": "true", "WS_TOKEN": "ignored-here"},
            virtual_display_override=":1",
        )
        assert built == {
            "host": "127.0.0.1",
            "port": 9224,
            "ws_path": "/camoufox",
            "headless": False,
            "virtual_display": ":1",
            "geoip": True,
            "os": "linux",
        }
    finally:
        if str(pkg_dir) in sys.path:
            sys.path.remove(str(pkg_dir))


def test_camoufox_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "camoufox" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "camoufox logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 1024


def test_camoufox_proxy_and_lsio_ui_mux(repo_root, tmp_path):
    pkg_dir = repo_root / "packages" / "camoufox"
    py_compile.compile(str(pkg_dir / "server.py"), doraise=True)
    py_compile.compile(str(pkg_dir / "config_builder.py"), doraise=True)

    node_bin = shutil.which("node")
    assert node_bin, "Node.js is required to test packages/camoufox/proxy.mjs"
    subprocess.run([node_bin, "--check", str(pkg_dir / "proxy.mjs")], check=True)

    cert_path = tmp_path / "cert.pem"
    key_path = tmp_path / "key.pem"
    openssl_bin = shutil.which("openssl")
    try:
        import datetime
        import ipaddress
        from cryptography import x509
        from cryptography.hazmat.primitives import hashes, serialization
        from cryptography.hazmat.primitives.asymmetric import rsa
        from cryptography.x509.oid import NameOID

        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        now = datetime.datetime.now(datetime.timezone.utc)
        cert = (
            x509.CertificateBuilder()
            .subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "camoufox")]))
            .issuer_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "camoufox")]))
            .public_key(key.public_key())
            .serial_number(x509.random_serial_number())
            .not_valid_before(now - datetime.timedelta(days=1))
            .not_valid_after(now + datetime.timedelta(days=30))
            .add_extension(
                x509.SubjectAlternativeName([x509.DNSName("localhost"), x509.IPAddress(ipaddress.ip_address("127.0.0.1"))]),
                critical=False,
            )
            .sign(key, hashes.SHA256())
        )
        key_path.write_bytes(key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
        cert_path.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    except ImportError:
        if openssl_bin:
            subprocess.run(
                [openssl_bin, "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", str(key_path), "-out", str(cert_path), "-days", "7", "-subj", "/CN=camoufox"],
                check=True,
                capture_output=True,
            )
        else:
            proxy_text = (pkg_dir / "proxy.mjs").read_text(encoding="utf-8")
            assert "UI_MUX_PORT" in proxy_text and "0x16" in proxy_text
            return

    class FakeHttpSelkies(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            body = f"LSIO_HTTP:{self.path}".encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            pass

    class FakeHttpsSelkies(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            body = f"LSIO_HTTPS:{self.path}".encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            pass

    http_ui = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FakeHttpSelkies)
    https_ui = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FakeHttpsSelkies)
    srv_ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    srv_ctx.load_cert_chain(certfile=str(cert_path), keyfile=str(key_path))
    https_ui.socket = srv_ctx.wrap_socket(https_ui.socket, server_side=True)

    threading.Thread(target=http_ui.serve_forever, daemon=True).start()
    threading.Thread(target=https_ui.serve_forever, daemon=True).start()

    free_ports = []
    for _ in range(2):
        with socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            free_ports.append(s.getsockname()[1])
    proxy_port, ui_mux_port = free_ports

    env = os.environ.copy()
    env.update(
        {
            "PROXY_BIND_HOST": "127.0.0.1",
            "PORT": str(proxy_port),
            "UI_MUX_PORT": str(ui_mux_port),
            "INTERNAL_WS_PORT": "1",
            "SELKIES_HTTP_PORT": str(http_ui.server_address[1]),
            "SELKIES_HTTPS_PORT": str(https_ui.server_address[1]),
            "VIEWER_ACTIVE": "true",
            "TLS_CERT_PATH": str(cert_path),
            "TLS_KEY_PATH": str(key_path),
            "WS_TOKEN": "",
        }
    )
    proc = subprocess.Popen([node_bin, str(pkg_dir / "proxy.mjs")], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        for port in (proxy_port, ui_mux_port):
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                try:
                    with socket.create_connection(("127.0.0.1", port), timeout=0.5):
                        break
                except OSError:
                    time.sleep(0.1)

        # 1. Playwright proxy /health + /viewer/ redirect on port 9223
        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/health")
        resp = conn.getresponse()
        health = json.loads(resp.read().decode())
        assert resp.status == 503 and health["viewerReady"] is True and health["uiMuxPort"] == ui_mux_port
        conn.close()

        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/viewer/", headers={"Host": f"192.168.4.19:{proxy_port}"})
        resp = conn.getresponse()
        resp.read()
        assert resp.status == 302
        assert resp.getheader("Location") == f"https://192.168.4.19:{ui_mux_port}/"
        conn.close()

        # 2. UI mux HTTP (Traefik upstream) -> LSIO HTTP :3000
        conn = http.client.HTTPConnection("127.0.0.1", ui_mux_port, timeout=5)
        conn.request("GET", "/")
        resp = conn.getresponse()
        assert resp.status == 200 and resp.read().decode() == "LSIO_HTTP:/"
        conn.close()

        # 3. UI mux HTTPS (direct LAN open) -> LSIO HTTPS :3001
        ctx = ssl._create_unverified_context()
        sconn = http.client.HTTPSConnection("127.0.0.1", ui_mux_port, context=ctx, timeout=5)
        sconn.request("GET", "/")
        sresp = sconn.getresponse()
        assert sresp.status == 200 and sresp.read().decode() == "LSIO_HTTPS:/"
        sconn.close()
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
        http_ui.shutdown()
        http_ui.server_close()
        https_ui.shutdown()
        https_ui.server_close()
