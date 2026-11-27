from PIL import Image


def test_dockhand_config_fields_and_single_container(get_app_config):
    config = get_app_config("dockhand")
    assert config["id"] == "dockhand"
    assert config["name"] == "Dockhand"
    assert config["port"] == 3180
    assert config["port"] != 3000
    assert config["tipi_version"] >= 2
    assert config["version"] != "latest"
    assert config["version"].startswith("v1.")
    assert config["dynamic_config"] is True
    assert config["exposable"] is True
    assert set(config["categories"]) == {"utilities", "development"}
    assert config["supported_architectures"] == ["amd64", "arm64"]

    form_fields = config.get("form_fields", [])
    fields_by_var = {f["env_variable"]: f for f in form_fields}

    assert "PUID" not in fields_by_var
    assert "PGID" not in fields_by_var
    assert "TZ" not in fields_by_var
    assert "POSTGRES_USER" not in fields_by_var
    assert "POSTGRES_DB" not in fields_by_var
    assert "POSTGRES_PASSWORD" not in fields_by_var
    assert "POSTGRES_CONTAINER_NAME" not in fields_by_var

    assert set(fields_by_var.keys()) == {"CONTAINER_NAME"}
    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "dockhand"


def test_dockhand_compose_single_container_no_ipam(get_app_config, get_app_compose):
    config = get_app_config("dockhand")
    compose = get_app_compose("dockhand")
    services = compose["services"]

    # Must stay single-service so Runtipi never injects dockhand_<store>_network IPAM subnets
    assert list(services.keys()) == ["dockhand"]

    dh = services["dockhand"]
    assert dh["image"] == f"fnsys/dockhand:{config['version']}"
    assert dh.get("container_name") == "${CONTAINER_NAME:-dockhand}"
    assert "depends_on" not in dh

    ports = [str(p) for p in dh.get("ports", [])]
    assert "${APP_PORT}:3000" in ports
    assert "3000:3000" not in ports

    dh_env = dh.get("environment", {})
    assert dh_env.get("TZ") == "${TZ}"
    assert "DATABASE_URL" not in dh_env
    assert "PUID" not in dh_env
    assert "PGID" not in dh_env

    volumes = [str(v) for v in dh.get("volumes", [])]
    assert "/var/run/docker.sock:/var/run/docker.sock" in volumes
    assert "${APP_DATA_DIR}/data/dockhand:/app/data" in volumes

    assert dh.get("networks") == ["tipi_main_network"]
    assert compose.get("networks") == {"tipi_main_network": {"external": True}}

    runtipi_cfg = dh.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 3000


def test_dockhand_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "dockhand" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "dockhand logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h == 1024
        r, g, b = img.getpixel((10, 10))
        assert 235 <= r <= 250 and 235 <= g <= 250 and 235 <= b <= 250
        cr, cg, cb = img.getpixel((512, 420))
        assert cr > 150 and cg < 120 and cb < 80
