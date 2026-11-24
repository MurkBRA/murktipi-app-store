import pytest

from PIL import Image

def test_spiral_pool_config_structure(get_app_config):
    config = get_app_config("spiral-pool")
    assert config["name"] == "Spiral Mining Pool"
    assert config["port"] == 1618
    assert "utilities" in config["categories"]
    assert "crypto" not in config["categories"], "Unrecognized category 'crypto' breaks Runtipi catalog listing"

    fields_by_var = {f["env_variable"]: f for f in config["form_fields"]}

    # Verify POOL_MODE field
    assert "POOL_MODE" in fields_by_var
    pool_mode_opts = [o["value"] for o in fields_by_var["POOL_MODE"]["options"]]
    assert "single" in pool_mode_opts
    assert "multi" in pool_mode_opts

    # Verify POOL_COIN field
    assert "POOL_COIN" in fields_by_var
    coin_opts = [o["value"] for o in fields_by_var["POOL_COIN"]["options"]]
    assert "bitcoin" in coin_opts
    assert "digibyte" in coin_opts
    assert "litecoin" in coin_opts
    assert "dogecoin" in coin_opts
    assert "bitcoincash" in coin_opts

    # Verify multi-coin flags
    assert "ENABLE_BTC" in fields_by_var
    assert "ENABLE_DGB" in fields_by_var
    assert "ENABLE_LTC" in fields_by_var
    assert "ENABLE_DOGE" in fields_by_var
    assert "MERGE_MINING_ENABLED" in fields_by_var

    # Verify credentials
    assert fields_by_var["DASHBOARD_ADMIN_PASSWORD"]["type"] == "password"
    assert fields_by_var["DB_PASSWORD"]["type"] == "password"

    # Verify optional per-service container_name fields
    assert fields_by_var["DASHBOARD_CONTAINER_NAME"]["default"] == "spiral-pool-dashboard"
    assert fields_by_var["STRATUM_CONTAINER_NAME"]["default"] == "spiral-pool-stratum"
    assert fields_by_var["POSTGRES_CONTAINER_NAME"]["default"] == "spiral-pool-postgres"

def test_spiral_pool_compose_services(get_app_compose):
    compose = get_app_compose("spiral-pool")
    services = compose["services"]

    assert "postgres" in services
    assert "stratum" in services
    assert "dashboard" in services

    # Postgres checks
    pg = services["postgres"]
    assert pg["image"] == "postgres:16-alpine"
    assert pg.get("container_name") == "${POSTGRES_CONTAINER_NAME:-spiral-pool-postgres}"
    assert "healthcheck" in pg
    assert pg["environment"]["POSTGRES_USER"] == "spiralstratum"

    # Stratum checks
    stratum = services["stratum"]
    assert stratum["image"] == "spiralpool/stratum:latest"
    assert stratum.get("container_name") == "${STRATUM_CONTAINER_NAME:-spiral-pool-stratum}"
    assert "postgres" in stratum["depends_on"]
    assert stratum["depends_on"]["postgres"]["condition"] == "service_healthy"

    # Dashboard checks
    dash = services["dashboard"]
    assert dash["image"] == "spiralpool/dashboard:latest"
    assert dash.get("container_name") == "${DASHBOARD_CONTAINER_NAME:-spiral-pool-dashboard}"
    assert dash["x-runtipi"]["is_main"] is True
    assert dash["x-runtipi"]["internal_port"] == 1618
    assert any("1618" in str(p) for p in dash.get("ports", []))

def test_spiral_pool_logo_is_jpg(apps_dir):
    logo = apps_dir / "spiral-pool" / "metadata" / "logo.jpg"
    assert logo.exists()
    with Image.open(logo) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h

