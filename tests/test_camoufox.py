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
    assert config["tipi_version"] >= 2
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
