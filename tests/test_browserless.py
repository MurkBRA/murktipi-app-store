from PIL import Image


def test_browserless_config_fields_and_optional_defaults(get_app_config):
    config = get_app_config("browserless")
    assert config["id"] == "browserless"
    assert config["name"] == "Browserless"
    assert config["port"] == 9222
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] != "latest"
    assert config["version"].startswith("v2.")
    assert config["tipi_version"] >= 1

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var

    expected_vars = {
        "BROWSERLESS_IMAGE",
        "CONTAINER_NAME",
        "SHM_SIZE",
        "MAX_CONCURRENT_SESSIONS",
        "CONNECTION_TIMEOUT",
        "PREBOOT_CHROME",
        "CHROME_REFRESH_TIME",
        "ENABLE_CORS",
        "DEMOING",
    }
    assert set(fields_by_var.keys()) == expected_vars

    img_field = fields_by_var["BROWSERLESS_IMAGE"]
    assert img_field["required"] is True
    assert img_field["default"] == "chrome"
    opt_values = [o["value"] for o in img_field["options"]]
    assert opt_values[0] == "chrome"
    assert set(opt_values) == {"chrome", "chromium", "firefox", "webkit", "multi"}

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "browserless"

    for opt_var in expected_vars - {"BROWSERLESS_IMAGE", "CONTAINER_NAME"}:
        field = fields_by_var[opt_var]
        assert field.get("required") is False, f"{opt_var} must be optional"
        assert "default" not in field, (
            f"{opt_var} must not force a default so leaving it blank omits it from container"
        )


def test_browserless_compose_structure_and_passthrough_env(get_app_config, get_app_compose):
    config = get_app_config("browserless")
    compose = get_app_compose("browserless")
    services = compose["services"]

    assert "browserless" in services
    svc = services["browserless"]

    assert svc.get("image") == f"ghcr.io/browserless/${{BROWSERLESS_IMAGE:-chrome}}:{config['version']}"
    assert svc.get("container_name") == "${CONTAINER_NAME:-browserless}"
    assert svc.get("restart") == "unless-stopped"
    assert svc.get("shm_size") == "${SHM_SIZE:-64m}"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:3000" in ports

    env = svc.get("environment", [])
    assert isinstance(env, list)
    for var in (
        "MAX_CONCURRENT_SESSIONS",
        "CONNECTION_TIMEOUT",
        "PREBOOT_CHROME",
        "CHROME_REFRESH_TIME",
        "ENABLE_CORS",
        "DEMOING",
    ):
        assert var in env, f"Expected pass-through env entry '{var}' without '='"
        assert not any(str(e).startswith(f"{var}=") for e in env), (
            f"'{var}' must not use '=' so unset values are omitted from container"
        )

    assert "TZ=${TZ}" in env
    assert not any(str(e).startswith("DEBUG=") for e in env)

    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 3000


def test_browserless_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "browserless" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "browserless logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w >= 512
