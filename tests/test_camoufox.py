import sys
from PIL import Image


def test_camoufox_config_fields_and_secrets(get_app_config):
    config = get_app_config("camoufox")
    assert config["id"] == "camoufox"
    assert config["name"] == "Camoufox"
    assert config["port"] == 9223
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] == "0.5.9"
    assert config["version"] != "latest"
    assert config["tipi_version"] >= 3
    assert set(config["categories"]) == {"development", "automation", "utilities"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var

    expected_vars = {
        "CONTAINER_NAME",
        "HEADLESS",
        "ENABLE_VIEWER",
        "VIEWER_RESOLUTION",
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
        "SHM_SIZE",
    }
    assert set(fields_by_var.keys()) == expected_vars

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "camoufox"

    headless = fields_by_var["HEADLESS"]
    assert headless["required"] is True
    assert headless["default"] == "virtual"
    assert [o["value"] for o in headless["options"]] == ["virtual", "true"]

    for secret_var in ("WS_TOKEN", "PROXY_PASSWORD"):
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
    assert svc.get("shm_size") == "${SHM_SIZE:-256m}"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:9223" in ports

    env = svc.get("environment", [])
    assert isinstance(env, list)
    assert "HEADLESS=${HEADLESS:-virtual}" in env
    assert "TZ=${TZ}" in env
    for var in (
        "ENABLE_VIEWER",
        "VIEWER_RESOLUTION",
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
    assert runtipi_cfg.get("internal_port") == 9223


def test_camoufox_package_dockerfile_workflow_and_builder(repo_root, get_app_config):
    config = get_app_config("camoufox")
    pkg_dir = repo_root / "packages" / "camoufox"
    for filename in ("Dockerfile", "requirements.txt", "config_builder.py", "proxy.mjs", "server.py"):
        assert (pkg_dir / filename).exists(), f"Missing packages/camoufox/{filename}"

    req_text = (pkg_dir / "requirements.txt").read_text(encoding="utf-8")
    assert f"camoufox[geoip]=={config['version']}" in req_text
    assert "selkies==" in req_text

    df_text = (pkg_dir / "Dockerfile").read_text(encoding="utf-8")
    assert "USER camoufox" in df_text
    assert "camoufox fetch" in df_text
    assert "openbox" in df_text
    assert "libgbm1" in df_text
    assert "SELKIES_PORT=8080" in df_text
    assert "EXPOSE 9223" in df_text

    workflow = repo_root / ".github" / "workflows" / "docker-camoufox.yml"
    assert workflow.exists()
    wf_text = workflow.read_text(encoding="utf-8")
    assert "ghcr.io" in wf_text
    assert "murkbra/camoufox" in wf_text
    assert "linux/amd64,linux/arm64" in wf_text
    assert "steps.appver.outputs.version" in wf_text

    sys.path.insert(0, str(pkg_dir))
    try:
        from config_builder import build_camoufox_kwargs, parse_viewer_enabled, parse_viewer_resolution

        assert parse_viewer_enabled({}) is True
        assert parse_viewer_enabled({"HEADLESS": "true"}) is False
        assert parse_viewer_resolution({}) == (1920, 1080)

        built = build_camoufox_kwargs(
            {"OS_TARGET": "linux", "GEOIP": "true", "WS_TOKEN": "ignored-here"},
            virtual_display_override=":99",
        )
        assert built == {
            "host": "127.0.0.1",
            "port": 9224,
            "ws_path": "/camoufox",
            "headless": False,
            "virtual_display": ":99",
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


def test_camoufox_proxy_single_port_http_https_and_viewer_routing(repo_root, tmp_path):
    import http.client
    import http.server
    import json
    import os
    import py_compile
    import shutil
    import socket
    import ssl
    import subprocess
    import threading
    import time

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
            assert "https.createServer" in proxy_text and "0x16" in proxy_text
            return

    class FakeSelkiesHandler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            body = f"SELKIES_OK:{self.path}:{self.headers.get('X-Forwarded-Proto', '')}".encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            pass

    selkies_srv = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FakeSelkiesHandler)
    selkies_port = selkies_srv.server_address[1]
    srv_thread = threading.Thread(target=selkies_srv.serve_forever, daemon=True)
    srv_thread.start()

    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        proxy_port = s.getsockname()[1]

    env = os.environ.copy()
    env.update(
        {
            "PROXY_BIND_HOST": "127.0.0.1",
            "PORT": str(proxy_port),
            "INTERNAL_WS_PORT": "1",
            "SELKIES_PORT": str(selkies_port),
            "VIEWER_ACTIVE": "true",
            "TLS_CERT_PATH": str(cert_path),
            "TLS_KEY_PATH": str(key_path),
            "WS_TOKEN": "",
        }
    )
    proc = subprocess.Popen([node_bin, str(pkg_dir / "proxy.mjs")], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            try:
                with socket.create_connection(("127.0.0.1", proxy_port), timeout=0.5):
                    break
            except OSError:
                time.sleep(0.1)

        # 1. Plain HTTP /health
        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/health")
        resp = conn.getresponse()
        health_http = json.loads(resp.read().decode())
        assert resp.status == 503 and health_http["tlsEnabled"] is True and health_http["viewerReady"] is True
        conn.close()

        # 2. Plain HTTP /viewer/ on LAN IP upgrades via 302 to HTTPS
        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/viewer/", headers={"Host": f"192.168.4.19:{proxy_port}"})
        resp = conn.getresponse()
        resp.read()
        assert resp.status == 302
        assert resp.getheader("Location") == f"https://192.168.4.19:{proxy_port}/viewer/"
        conn.close()

        # 3. Reverse-proxy HTTP with X-Forwarded-Proto: https serves Selkies directly without redirect
        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/viewer/", headers={"Host": "camoufox.example.com", "X-Forwarded-Proto": "https"})
        resp = conn.getresponse()
        body = resp.read().decode()
        assert resp.status == 200 and body.startswith("SELKIES_OK:/viewer/:https")
        conn.close()

        # 4. Direct HTTPS on the exact same port serves both /health and /viewer/
        ctx = ssl._create_unverified_context()
        sconn = http.client.HTTPSConnection("127.0.0.1", proxy_port, context=ctx, timeout=5)
        sconn.request("GET", "/viewer/")
        sresp = sconn.getresponse()
        sbody = sresp.read().decode()
        assert sresp.status == 200 and sbody == "SELKIES_OK:/viewer/:https"
        sconn.close()
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
        selkies_srv.shutdown()
        selkies_srv.server_close()
