from PIL import Image


def test_email_oauth2_proxy_ui_config_fields(get_app_config):
    config = get_app_config("email-oauth2-proxy-ui")
    assert config["id"] == "email-oauth2-proxy-ui"
    assert config["name"] == "Email OAuth2 Proxy UI"
    assert config["port"] == 8090
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert set(["utilities", "security"]).issubset(set(config["categories"]))

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var

    assert set(fields_by_var.keys()) == {
        "PROXY_CONFIG_DIR",
        "PROXY_COMPOSE_DIR",
        "PROXY_HOST",
        "PROXY_CONTAINER",
        "ADMIN_USER",
        "ADMIN_PASSWORD",
    }

    assert fields_by_var["PROXY_CONFIG_DIR"]["required"] is True
    assert fields_by_var["PROXY_COMPOSE_DIR"]["required"] is False
    assert fields_by_var["PROXY_HOST"]["required"] is True
    assert fields_by_var["PROXY_HOST"]["default"] == "email-oauth2-proxy"
    assert fields_by_var["PROXY_CONTAINER"]["default"] == "email-oauth2-proxy"
    assert fields_by_var["ADMIN_USER"]["default"] == "admin"
    assert fields_by_var["ADMIN_PASSWORD"]["type"] == "password"
    assert fields_by_var["ADMIN_PASSWORD"]["required"] is False


def test_email_oauth2_proxy_ui_compose_structure(get_app_compose):
    compose = get_app_compose("email-oauth2-proxy-ui")
    services = compose["services"]

    assert "email-oauth2-proxy-ui" in services
    svc = services["email-oauth2-proxy-ui"]

    assert svc.get("image") == "golang:1.24-alpine"
    assert svc.get("container_name") == "email-oauth2-proxy-ui"
    assert svc.get("restart") == "unless-stopped"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:8090" in ports

    volumes = [str(v) for v in svc.get("volumes", [])]
    assert "${APP_DATA_DIR}/data/bin:/app-cache:rw" in volumes
    assert "${PROXY_CONFIG_DIR}:/config:rw" in volumes
    assert "${PROXY_COMPOSE_DIR:-/tmp/email-oauth2-proxy-compose}:/compose:rw" in volumes
    assert "/var/run/docker.sock:/var/run/docker.sock" in volumes

    env = svc.get("environment", {})
    assert env.get("TZ") == "${TZ}"
    assert env.get("LISTEN_ADDR") == ":8090"
    assert env.get("CONFIG_DIR") == "/config"
    assert env.get("COMPOSE_DIR") == "/compose"
    assert env.get("PROXY_HOST") == "${PROXY_HOST:-email-oauth2-proxy}"
    assert env.get("PROXY_CONTAINER") == "${PROXY_CONTAINER:-email-oauth2-proxy}"
    assert env.get("ADMIN_USER") == "${ADMIN_USER:-admin}"
    assert env.get("ADMIN_PASSWORD") == "${ADMIN_PASSWORD:-}"

    assert "healthcheck" in svc
    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 8090


def test_email_oauth2_proxy_ui_go_source_bundled(repo_root, get_app_compose):
    src_dir = repo_root / "packages" / "email-oauth2-proxy-ui"
    assert (src_dir / "go.mod").exists()
    assert (src_dir / "main.go").exists()
    assert (src_dir / "internal").is_dir()
    assert (src_dir / "web").is_dir()

    compose = get_app_compose("email-oauth2-proxy-ui")
    cmd = " ".join(str(x) for x in compose["services"]["email-oauth2-proxy-ui"].get("command", []))
    assert "packages/email-oauth2-proxy-ui" in cmd
    assert "codeload.github.com/MurkBRA/murktipi-app-store" in cmd


def test_email_oauth2_proxy_ui_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "email-oauth2-proxy-ui" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "email-oauth2-proxy-ui logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w >= 512
