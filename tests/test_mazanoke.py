from PIL import Image


def test_mazanoke_config_fields_and_optional_auth(get_app_config):
    config = get_app_config("mazanoke")
    assert config["id"] == "mazanoke"
    assert config["name"] == "MAZANOKE"
    assert config["port"] == 3474
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] != "latest"
    assert config["version"].startswith("v1.")
    assert config["tipi_version"] >= 1
    assert set(config["categories"]) == {"utilities", "photography", "media"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var
    assert set(fields_by_var.keys()) == {"CONTAINER_NAME", "USERNAME", "PASSWORD"}

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "mazanoke"

    assert fields_by_var["USERNAME"]["type"] == "text"
    assert fields_by_var["USERNAME"]["required"] is False
    assert "default" not in fields_by_var["USERNAME"]

    assert fields_by_var["PASSWORD"]["type"] == "password"
    assert fields_by_var["PASSWORD"]["required"] is False
    assert "default" not in fields_by_var["PASSWORD"]


def test_mazanoke_compose_structure_and_passthrough_auth(get_app_config, get_app_compose):
    config = get_app_config("mazanoke")
    compose = get_app_compose("mazanoke")
    services = compose["services"]

    assert "mazanoke" in services
    svc = services["mazanoke"]
    assert svc.get("image") == f"ghcr.io/civilblur/mazanoke:{config['version']}"
    assert svc.get("container_name") == "${CONTAINER_NAME:-mazanoke}"
    assert svc.get("restart") == "unless-stopped"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:80" in ports

    env = svc.get("environment", [])
    assert isinstance(env, list)
    for var in ("USERNAME", "PASSWORD"):
        assert var in env
        assert not any(str(e).startswith(f"{var}=") for e in env)
    assert "TZ=${TZ}" in env

    assert "healthcheck" in svc
    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 80


def test_mazanoke_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "mazanoke" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "mazanoke logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 1024
