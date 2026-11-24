from PIL import Image


def test_dockhand_config_fields_and_random_password(get_app_config):
    config = get_app_config("dockhand")
    assert config["id"] == "dockhand"
    assert config["name"] == "Dockhand"
    assert config["port"] == 3180
    assert config["port"] != 3000
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

    assert set(fields_by_var.keys()) == {
        "POSTGRES_USER",
        "POSTGRES_DB",
        "POSTGRES_PASSWORD",
        "CONTAINER_NAME",
        "POSTGRES_CONTAINER_NAME",
    }

    assert fields_by_var["POSTGRES_USER"]["required"] is True
    assert "default" not in fields_by_var["POSTGRES_USER"]

    assert fields_by_var["POSTGRES_DB"]["required"] is True
    assert fields_by_var["POSTGRES_DB"]["default"] == "dockhand"

    pw = fields_by_var["POSTGRES_PASSWORD"]
    assert pw["type"] == "random"
    assert pw.get("encoding") == "hex"
    assert pw.get("min", 0) >= 32

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "dockhand"
    assert fields_by_var["POSTGRES_CONTAINER_NAME"]["required"] is False
    assert fields_by_var["POSTGRES_CONTAINER_NAME"]["default"] == "dockhand-postgres"


def test_dockhand_compose_structure_and_postgres_integration(get_app_config, get_app_compose):
    config = get_app_config("dockhand")
    compose = get_app_compose("dockhand")
    services = compose["services"]

    assert "postgres" in services
    assert "dockhand" in services

    pg = services["postgres"]
    assert pg["image"] == "postgres:16-alpine"
    assert pg.get("container_name") == "${POSTGRES_CONTAINER_NAME:-dockhand-postgres}"
    assert "healthcheck" in pg
    pg_env = pg.get("environment", {})
    assert pg_env.get("POSTGRES_USER") == "${POSTGRES_USER:-dockhand}"
    assert pg_env.get("POSTGRES_PASSWORD") == "${POSTGRES_PASSWORD}"
    assert pg_env.get("POSTGRES_DB") == "${POSTGRES_DB:-dockhand}"
    assert "${APP_DATA_DIR}/data/postgres:/var/lib/postgresql/data" in [str(v) for v in pg.get("volumes", [])]

    dh = services["dockhand"]
    assert dh["image"] == f"fnsys/dockhand:{config['version']}"
    assert dh.get("container_name") == "${CONTAINER_NAME:-dockhand}"
    assert dh["depends_on"]["postgres"]["condition"] == "service_healthy"

    ports = [str(p) for p in dh.get("ports", [])]
    assert "${APP_PORT}:3000" in ports
    assert "3000:3000" not in ports

    dh_env = dh.get("environment", {})
    assert dh_env.get("TZ") == "${TZ}"
    assert (
        dh_env.get("DATABASE_URL")
        == "postgres://${POSTGRES_USER:-dockhand}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-dockhand}"
    )
    assert "PUID" not in dh_env
    assert "PGID" not in dh_env

    volumes = [str(v) for v in dh.get("volumes", [])]
    assert "/var/run/docker.sock:/var/run/docker.sock" in volumes
    assert "${APP_DATA_DIR}/data/dockhand:/app/data" in volumes

    runtipi_cfg = dh.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 3000


def test_dockhand_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "dockhand" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "dockhand logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w >= 512
