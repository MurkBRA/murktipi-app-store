from PIL import Image


def test_pic_smaller_config_fields(get_app_config):
    config = get_app_config("pic-smaller")
    assert config["id"] == "pic-smaller"
    assert config["name"] == "Pic Smaller"
    assert config["port"] == 3002
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert config["version"] == "1.1.0"
    assert config["version"] != "latest"
    assert config["tipi_version"] >= 1
    assert set(config["categories"]) == {"utilities", "photography", "media"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var
    assert set(fields_by_var.keys()) == {"CONTAINER_NAME"}
    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "pic-smaller"


def test_pic_smaller_compose_and_workflow(repo_root, get_app_config, get_app_compose):
    config = get_app_config("pic-smaller")
    compose = get_app_compose("pic-smaller")
    services = compose["services"]

    assert "pic-smaller" in services
    svc = services["pic-smaller"]
    assert svc.get("image") == f"ghcr.io/murkbra/pic-smaller:{config['version']}"
    assert svc.get("container_name") == "${CONTAINER_NAME:-pic-smaller}"
    assert svc.get("restart") == "unless-stopped"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:3000" in ports

    env = svc.get("environment", [])
    assert isinstance(env, list)
    assert "TZ=${TZ}" in env

    assert "healthcheck" in svc
    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 3000

    workflow = repo_root / ".github" / "workflows" / "docker-pic-smaller.yml"
    assert workflow.exists(), "docker-pic-smaller.yml workflow must exist"
    wf_text = workflow.read_text(encoding="utf-8")
    assert "ghcr.io" in wf_text
    assert "murkbra/pic-smaller" in wf_text
    assert "joye61/pic-smaller" in wf_text
    assert "linux/amd64,linux/arm64" in wf_text
    assert "steps.appver.outputs.version" in wf_text


def test_pic_smaller_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "pic-smaller" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "pic-smaller logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 1024
