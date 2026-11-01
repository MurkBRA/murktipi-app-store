import pytest
from PIL import Image

def test_technitium_config_has_strictly_3_variables(get_app_config):
    config = get_app_config("technitium")
    form_fields = config.get("form_fields", [])

    # Must contain strictly the 3 variables requested by the user
    assert len(form_fields) == 3, f"Technitium must have strictly 3 form_fields, found {len(form_fields)}"
    fields_by_var = {f["env_variable"]: f for f in form_fields}

    assert "DNS_SERVER_DOMAIN" in fields_by_var
    assert fields_by_var["DNS_SERVER_DOMAIN"]["default"] == "technitium-dns"

    assert "DNS_SERVER_RECURSION" in fields_by_var
    assert fields_by_var["DNS_SERVER_RECURSION"]["default"] == "Allow"

    assert "DNS_SERVER_ADMIN_PASSWORD" in fields_by_var
    assert fields_by_var["DNS_SERVER_ADMIN_PASSWORD"]["default"] == "senha"

def test_technitium_compose_structure(get_app_compose):
    compose = get_app_compose("technitium")
    services = compose["services"]

    # Service name must be technitium-dns
    assert "technitium-dns" in services
    service = services["technitium-dns"]

    # Check container_name, image, restart
    assert service.get("container_name") == "technitium-dns"
    assert service.get("image") == "technitium/dns-server:latest"
    assert service.get("restart") == "unless-stopped"

    # Check standard ports
    ports = [str(p) for p in service.get("ports", [])]
    assert any("53:53/tcp" in p for p in ports), "Missing port 53:53/tcp"
    assert any("53:53/udp" in p for p in ports), "Missing port 53:53/udp"
    assert any("5380" in p for p in ports), "Missing port 5380"

    # Environment must contain strictly the 3 variables
    env = service.get("environment", [])
    env_str = str(env)
    assert "DNS_SERVER_DOMAIN" in env_str
    assert "DNS_SERVER_RECURSION" in env_str
    assert "DNS_SERVER_ADMIN_PASSWORD" in env_str
    assert "EXTRA_VAR_" not in env_str
    assert "EXTRA_CUSTOM_VARS" not in env_str

def test_technitium_logo_is_jpg(apps_dir):
    logo_path = apps_dir / "technitium" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "technitium logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h
        assert w >= 512
