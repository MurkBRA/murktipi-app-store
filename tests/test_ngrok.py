from PIL import Image


def test_ngrok_config_fields_and_secrets(get_app_config):
    config = get_app_config("ngrok")
    assert config["id"] == "ngrok"
    assert config["name"] == "Ngrok"
    assert config["port"] == 4040
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] == "3.39.13"
    assert config["version"] != "latest"
    assert config["tipi_version"] >= 3
    assert set(config["categories"]) == {"network", "utilities", "development"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    fields = {f["env_variable"]: f for f in config.get("form_fields", [])}
    assert set(fields.keys()) == {
        "NGROK_AUTHTOKEN",
        "NGROK_URL",
        "NGROK_TARGET",
        "CONTAINER_NAME",
    }

    assert fields["NGROK_AUTHTOKEN"]["type"] == "password"
    assert fields["NGROK_AUTHTOKEN"]["required"] is True
    assert "default" not in fields["NGROK_AUTHTOKEN"]

    assert fields["NGROK_URL"]["required"] is True
    assert "default" not in fields["NGROK_URL"]

    assert fields["NGROK_TARGET"]["required"] is False
    assert "default" not in fields["NGROK_TARGET"]

    assert fields["CONTAINER_NAME"]["required"] is False
    assert fields["CONTAINER_NAME"]["default"] == "ngrok"


def test_ngrok_compose_official_single_container(repo_root, get_app_config, get_app_compose):
    config = get_app_config("ngrok")
    compose = get_app_compose("ngrok")
    services = compose["services"]

    # Must stay single-service so Runtipi never injects a per-app IPAM subnet network or depends_on
    assert list(services.keys()) == ["ngrok"]
    ng = services["ngrok"]
    assert ng.get("image") == f"ngrok/ngrok:{config['version']}-alpine"
    assert ng.get("container_name") == "${CONTAINER_NAME:-ngrok}"
    assert ng.get("command") == "http --url=${NGROK_URL} ${NGROK_TARGET:-80}"
    assert [str(p) for p in ng.get("ports", [])] == ["${APP_PORT}:4040"]
    assert "volumes" not in ng
    assert "depends_on" not in ng
    assert "healthcheck" in ng
    assert ng.get("x-runtipi", {}).get("is_main") is True
    assert ng.get("x-runtipi", {}).get("internal_port") == 4040

    ng_env = [str(e) for e in ng.get("environment", [])]
    assert "TZ=${TZ}" in ng_env
    assert "NGROK_AUTHTOKEN=${NGROK_AUTHTOKEN}" in ng_env

    assert not (repo_root / "packages" / "ngrok").exists()
    assert not (repo_root / ".github" / "workflows" / "docker-ngrok.yml").exists()


def test_ngrok_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "ngrok" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "ngrok logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 512
