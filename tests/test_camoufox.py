import http.client
import json
import os
import py_compile
import shutil
import socket
import subprocess
import sys
import threading
import time
from PIL import Image


def test_camoufox_config_fields_and_secrets(get_app_config):
    config = get_app_config("camoufox")
    assert config["id"] == "camoufox"
    assert config["name"] == "Camoufox"
    assert config["port"] == 9223
    assert config.get("https") is None
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] == "0.5.9"
    assert config["version"] != "latest"
    assert config["tipi_version"] >= 9
    assert set(config["categories"]) == {"development", "automation", "utilities"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var
    for removed in (
        "CUSTOM_USER",
        "PASSWORD",
        "AUTOSTART_BROWSER",
        "PUID",
        "PGID",
        "PROXY_SERVER",
        "PROXY_USERNAME",
        "PROXY_PASSWORD",
    ):
        assert removed not in fields_by_var

    expected_vars = {
        "CONTAINER_NAME",
        "WS_TOKEN",
        "HEADLESS",
        "SLIM_FONTS",
        "GEOIP",
        "OS_TARGET",
        "LOCALE",
        "BLOCK_IMAGES",
        "BLOCK_WEBRTC",
        "DISABLE_COOP",
        "SHM_SIZE",
    }
    assert set(fields_by_var.keys()) == expected_vars

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "camoufox"
    assert fields_by_var["SHM_SIZE"]["label"] == "shm_size"

    headless = fields_by_var["HEADLESS"]
    assert headless["required"] is True
    assert headless["default"] == "virtual"
    assert [o["value"] for o in headless["options"]] == ["virtual", "true"]

    assert fields_by_var["WS_TOKEN"]["type"] == "password"
    assert fields_by_var["WS_TOKEN"]["required"] is False
    assert "default" not in fields_by_var["WS_TOKEN"]

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
    assert ports == ["${APP_PORT}:9223"]

    volumes = [str(v) for v in svc.get("volumes", [])]
    assert "${APP_DATA_DIR}/data/config:/config" in volumes

    env = svc.get("environment", [])
    assert isinstance(env, list)
    assert "HEADLESS=${HEADLESS:-virtual}" in env
    assert "TZ=${TZ}" in env
    for var in (
        "SLIM_FONTS",
        "WS_TOKEN",
        "GEOIP",
        "OS_TARGET",
        "LOCALE",
        "BLOCK_IMAGES",
        "BLOCK_WEBRTC",
        "DISABLE_COOP",
    ):
        assert var in env
        assert not any(str(e).startswith(f"{var}=") for e in env)

    hc = svc.get("healthcheck", {})
    assert "127.0.0.1:9223/health" in " ".join(str(x) for x in hc.get("test", []))
    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 9223


def test_camoufox_package_dockerfile_workflow_and_builder(repo_root, get_app_config):
    config = get_app_config("camoufox")
    pkg_dir = repo_root / "packages" / "camoufox"
    for filename in (
        "Dockerfile",
        "requirements.txt",
        "config_builder.py",
        "proxy.mjs",
        "server.py",
    ):
        assert (pkg_dir / filename).exists(), f"Missing packages/camoufox/{filename}"

    for obsolete in ("icon.png", "root"):
        assert not (pkg_dir / obsolete).exists(), f"Obsolete Selkies artifact still present: packages/camoufox/{obsolete}"

    req_text = (pkg_dir / "requirements.txt").read_text(encoding="utf-8")
    assert f"camoufox[geoip]=={config['version']}" in req_text
    assert "selkies" not in req_text.lower()

    df_text = (pkg_dir / "Dockerfile").read_text(encoding="utf-8")
    assert "FROM python:3.12-slim" in df_text
    assert "baseimage-selkies" not in df_text
    assert "ensure_fpgen_model" in df_text
    assert "xvfb" in df_text
    assert "camoufox fetch" not in df_text
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
        import config_builder

        assert not hasattr(config_builder, "parse_viewer_enabled")
        assert config_builder.parse_slim_fonts({"SLIM_FONTS": "true"}) is True
        assert config_builder.resolve_uid_gid({"PUID": "1000", "PGID": "1000"}) == (1000, 1000)

        built = config_builder.build_camoufox_kwargs(
            {"OS_TARGET": "linux", "GEOIP": "true", "WS_TOKEN": "ignored-here"}
        )
        assert built == {
            "host": "127.0.0.1",
            "port": 9224,
            "ws_path": "/camoufox",
            "headless": "virtual",
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


def test_camoufox_proxy_http_json_and_ws_token_forwarding(repo_root):
    pkg_dir = repo_root / "packages" / "camoufox"
    py_compile.compile(str(pkg_dir / "server.py"), doraise=True)
    py_compile.compile(str(pkg_dir / "config_builder.py"), doraise=True)

    node_bin = shutil.which("node")
    assert node_bin, "Node.js is required to test packages/camoufox/proxy.mjs"
    subprocess.run([node_bin, "--check", str(pkg_dir / "proxy.mjs")], check=True)

    captured_requests = []
    upstream_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    upstream_sock.bind(("127.0.0.1", 0))
    upstream_sock.listen(5)
    internal_port = upstream_sock.getsockname()[1]
    stop_upstream = threading.Event()

    def serve_upstream():
        upstream_sock.settimeout(0.2)
        while not stop_upstream.is_set():
            try:
                conn, _ = upstream_sock.accept()
            except socket.timeout:
                continue
            except OSError:
                break
            try:
                data = conn.recv(4096)
                if data:
                    captured_requests.append(data.decode("utf-8", errors="replace"))
                    conn.sendall(
                        b"HTTP/1.1 101 Switching Protocols\r\n"
                        b"Upgrade: websocket\r\n"
                        b"Connection: Upgrade\r\n\r\n"
                    )
            finally:
                conn.close()

    upstream_thread = threading.Thread(target=serve_upstream, daemon=True)
    upstream_thread.start()

    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        proxy_port = s.getsockname()[1]

    env = os.environ.copy()
    env.update(
        {
            "PROXY_BIND_HOST": "127.0.0.1",
            "PORT": str(proxy_port),
            "INTERNAL_WS_PORT": str(internal_port),
            "INTERNAL_WS_PATH": "/camoufox",
            "WS_TOKEN": "test-secret",
            "HEADLESS": "virtual",
            "GEOIP": "false",
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

        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/health")
        resp = conn.getresponse()
        health = json.loads(resp.read().decode())
        assert resp.status == 200 and health["ready"] is True and health["engine"] == "camoufox"
        assert "viewerEnabled" not in health
        conn.close()

        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/json")
        assert conn.getresponse().status == 401
        conn.close()

        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/json?token=test-secret")
        resp = conn.getresponse()
        info = json.loads(resp.read().decode())
        assert resp.status == 200 and info["wsEndpointPath"] == "/test-secret"
        conn.close()

        conn = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=5)
        conn.request("GET", "/")
        resp = conn.getresponse()
        html = resp.read().decode()
        assert resp.status == 200
        assert "Camoufox Playwright Server" in html
        assert "Selkies" not in html
        conn.close()

        # Unauthorized WebSocket upgrade rejected
        with socket.create_connection(("127.0.0.1", proxy_port), timeout=5) as ws_unauth:
            ws_unauth.sendall(
                b"GET / HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"
            )
            assert b"401 Unauthorized" in ws_unauth.recv(1024)

        # Authorized WebSocket upgrade strips query token and rewrites target to /camoufox
        with socket.create_connection(("127.0.0.1", proxy_port), timeout=5) as ws_auth:
            ws_auth.sendall(
                b"GET /?token=test-secret HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"
            )
            assert b"101 Switching Protocols" in ws_auth.recv(1024)

        assert any(req.startswith("GET /camoufox HTTP/1.1") for req in captured_requests)
    finally:
        stop_upstream.set()
        upstream_sock.close()
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
