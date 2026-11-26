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
    assert config["tipi_version"] >= 2
    assert set(config["categories"]) == {"network", "utilities", "development"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    fields = {f["env_variable"]: f for f in config.get("form_fields", [])}
    assert set(fields.keys()) == {
        "NGROK_AUTHTOKEN",
        "NGROK_URL",
        "ENABLE_NGINX_FILTER",
        "NGROK_TARGET",
        "NGINX_CONF_BASE64",
        "CONTAINER_NAME",
    }

    assert fields["NGROK_AUTHTOKEN"]["type"] == "password"
    assert fields["NGROK_AUTHTOKEN"]["required"] is True
    assert "default" not in fields["NGROK_AUTHTOKEN"]

    assert fields["NGROK_URL"]["required"] is True
    assert "default" not in fields["NGROK_URL"]

    flt = fields["ENABLE_NGINX_FILTER"]
    assert flt["required"] is True
    assert flt["default"] == "true"
    assert [o["value"] for o in flt["options"]] == ["true", "false"]

    for opt_var in ("NGROK_TARGET", "NGINX_CONF_BASE64"):
        assert fields[opt_var]["required"] is False
        assert "default" not in fields[opt_var]


def test_ngrok_compose_single_service_workflow_and_package(repo_root, get_app_config, get_app_compose):
    config = get_app_config("ngrok")
    compose = get_app_compose("ngrok")
    services = compose["services"]

    # Must stay single-service so Runtipi never injects a per-app IPAM subnet network or depends_on
    assert list(services.keys()) == ["ngrok"]
    ng = services["ngrok"]
    assert ng.get("image") == f"ghcr.io/murkbra/ngrok:{config['version']}"
    assert ng.get("container_name") == "${CONTAINER_NAME:-ngrok}"
    assert [str(p) for p in ng.get("ports", [])] == ["${APP_PORT}:4040"]
    assert "volumes" not in ng
    assert "depends_on" not in ng
    assert "command" not in ng
    assert "healthcheck" in ng
    assert ng.get("x-runtipi", {}).get("is_main") is True
    assert ng.get("x-runtipi", {}).get("internal_port") == 4040

    ng_env = [str(e) for e in ng.get("environment", [])]
    assert "NGROK_AUTHTOKEN=${NGROK_AUTHTOKEN}" in ng_env
    assert "NGROK_URL=${NGROK_URL}" in ng_env
    assert "ENABLE_NGINX_FILTER=${ENABLE_NGINX_FILTER:-true}" in ng_env
    assert "NGROK_TARGET" in ng_env
    assert "NGINX_CONF_BASE64" in ng_env

    pkg_dir = repo_root / "packages" / "ngrok"
    for fn in ("Dockerfile", "default.conf", "entrypoint.sh"):
        assert (pkg_dir / fn).exists(), f"Missing packages/ngrok/{fn}"

    wf_path = repo_root / ".github" / "workflows" / "docker-ngrok.yml"
    assert wf_path.exists()
    wf_text = wf_path.read_text(encoding="utf-8")
    assert "murkbra/ngrok" in wf_text
    assert "linux/amd64,linux/arm64" in wf_text
    assert "steps.appver.outputs.version" in wf_text


def test_ngrok_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "ngrok" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "ngrok logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 512
