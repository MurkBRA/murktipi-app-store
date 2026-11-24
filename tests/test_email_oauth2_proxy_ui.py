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
        "PROXY_STORE_ID",
        "PROXY_HOST",
        "PROXY_CONTAINER",
        "ADMIN_USER",
        "ADMIN_PASSWORD",
        "CONTAINER_NAME",
    }

    assert fields_by_var["PROXY_STORE_ID"]["required"] is True
    assert fields_by_var["PROXY_STORE_ID"]["default"] == "migrated"
    assert fields_by_var["PROXY_HOST"]["required"] is True
    assert fields_by_var["PROXY_HOST"]["default"] == "email-oauth2-proxy"
    assert fields_by_var["PROXY_CONTAINER"]["default"] == "email-oauth2-proxy"
    assert fields_by_var["ADMIN_USER"]["default"] == "admin"
    assert fields_by_var["ADMIN_PASSWORD"]["type"] == "password"
    assert fields_by_var["ADMIN_PASSWORD"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "email-oauth2-proxy-ui"


def test_email_oauth2_proxy_ui_compose_structure(get_app_config, get_app_compose):
    config = get_app_config("email-oauth2-proxy-ui")
    compose = get_app_compose("email-oauth2-proxy-ui")
    services = compose["services"]

    assert "email-oauth2-proxy-ui" in services
    svc = services["email-oauth2-proxy-ui"]

    assert config["version"] != "latest"
    assert config["tipi_version"] >= 2
    assert svc.get("image") == f"ghcr.io/murkbra/email-oauth2-proxy-ui:{config['version']}"
    assert svc.get("container_name") == "${CONTAINER_NAME:-email-oauth2-proxy-ui}"
    assert svc.get("restart") == "unless-stopped"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:8090" in ports

    volumes = [str(v) for v in svc.get("volumes", [])]
    assert "${ROOT_FOLDER_HOST}/app-data/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy/data/config:/config:rw" in volumes
    assert "${ROOT_FOLDER_HOST}/user-config/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy:/compose:rw" in volumes
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
    assert svc["healthcheck"]["test"] == ["CMD", "/email-oauth2-proxy-ui", "-healthcheck"]
    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 8090


def test_email_oauth2_proxy_ui_dockerfile_and_workflow(repo_root):
    src_dir = repo_root / "packages" / "email-oauth2-proxy-ui"
    dockerfile = src_dir / "Dockerfile"
    workflow = repo_root / ".github" / "workflows" / "docker-email-oauth2-proxy-ui.yml"

    assert (src_dir / "go.mod").exists()
    assert (src_dir / "main.go").exists()
    assert dockerfile.exists()
    assert workflow.exists()

    df_text = dockerfile.read_text(encoding="utf-8")
    assert "FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder" in df_text
    assert "FROM scratch" in df_text
    assert "GOOS=${TARGETOS} GOARCH=${TARGETARCH}" in df_text

    wf_text = workflow.read_text(encoding="utf-8")
    assert "ghcr.io" in wf_text
    assert "murkbra/email-oauth2-proxy-ui" in wf_text
    assert "setup-qemu-action" not in wf_text


def test_email_oauth2_proxy_ui_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "email-oauth2-proxy-ui" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "email-oauth2-proxy-ui logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w >= 512
