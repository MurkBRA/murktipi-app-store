from PIL import Image


def test_imgcompress_config_fields_and_optional_defaults(get_app_config):
    config = get_app_config("imgcompress")
    assert config["id"] == "imgcompress"
    assert config["name"] == "ImgCompress"
    assert config["port"] == 3001
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] != "latest"
    assert config["version"] == "1.0.0"
    assert config["tipi_version"] >= 1
    assert set(config["categories"]) == {"utilities", "photography", "media"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var

    expected_vars = {
        "CONTAINER_NAME",
        "MAX_UPLOAD_SIZE_MB",
        "WEB_WORKERS",
        "DISABLE_LOGO",
        "DISABLE_STORAGE_MANAGEMENT",
        "DISABLE_UPDATE_CHECK",
        "NOTIFY_URL",
        "PUBLIC_BASE_URL",
    }
    assert set(fields_by_var.keys()) == expected_vars

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "imgcompress"

    for opt_var in expected_vars - {"CONTAINER_NAME"}:
        field = fields_by_var[opt_var]
        assert field.get("required") is False, f"{opt_var} must be optional"
        assert "default" not in field, (
            f"{opt_var} must not force a default so leaving it blank omits it from container"
        )


def test_imgcompress_compose_structure_healthcheck_and_passthrough_env(get_app_config, get_app_compose):
    config = get_app_config("imgcompress")
    compose = get_app_compose("imgcompress")
    services = compose["services"]

    assert "imgcompress" in services
    svc = services["imgcompress"]

    assert svc.get("image") == f"karimz1/imgcompress:{config['version']}"
    assert svc.get("container_name") == "${CONTAINER_NAME:-imgcompress}"
    assert svc.get("restart") == "unless-stopped"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:5000" in ports

    env = svc.get("environment", [])
    assert isinstance(env, list)
    for var in (
        "MAX_UPLOAD_SIZE_MB",
        "WEB_WORKERS",
        "DISABLE_LOGO",
        "DISABLE_STORAGE_MANAGEMENT",
        "DISABLE_UPDATE_CHECK",
        "NOTIFY_URL",
        "PUBLIC_BASE_URL",
    ):
        assert var in env, f"Expected pass-through env entry '{var}' without '='"
        assert not any(str(e).startswith(f"{var}=") for e in env), (
            f"'{var}' must not use '=' so unset values are omitted from container"
        )

    assert "TZ=${TZ}" in env
    assert "healthcheck" in svc
    assert svc["healthcheck"].get("test") == ["CMD", "python", "/container/healthcheck.py"]

    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 5000


def test_imgcompress_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "imgcompress" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "imgcompress logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 1024
