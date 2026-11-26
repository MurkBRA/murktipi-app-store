import base64
import re
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
    assert config["tipi_version"] >= 1
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
        "FILTERS_CONTAINER_NAME",
    }

    assert fields["NGROK_AUTHTOKEN"]["type"] == "password"
    assert fields["NGROK_AUTHTOKEN"]["required"] is True
    assert "default" not in fields["NGROK_AUTHTOKEN"]

    flt = fields["ENABLE_NGINX_FILTER"]
    assert flt["required"] is True
    assert flt["default"] == "true"
    assert [o["value"] for o in flt["options"]] == ["true", "false"]

    for opt_var in ("NGROK_URL", "NGROK_TARGET", "NGINX_CONF_BASE64"):
        assert fields[opt_var]["required"] is False
        assert "default" not in fields[opt_var]


def test_ngrok_compose_structure_optional_filter_and_base64(get_app_config, get_app_compose):
    config = get_app_config("ngrok")
    compose = get_app_compose("ngrok")
    services = compose["services"]

    assert set(services.keys()) == {"ngrok-filtros", "ngrok"}

    flt = services["ngrok-filtros"]
    assert flt.get("image") == "nginx:1.31.10-alpine"
    assert flt.get("container_name") == "${FILTERS_CONTAINER_NAME:-ngrok-filtros}"
    assert "volumes" not in flt

    flt_env = [str(e) for e in flt.get("environment", [])]
    b64_entries = [e for e in flt_env if e.startswith("NGINX_CONF_B64=")]
    assert len(b64_entries) == 1
    m = re.match(r"^NGINX_CONF_B64=\$\{NGINX_CONF_BASE64:-([A-Za-z0-9+/=]+)\}$", b64_entries[0])
    assert m, f"Unexpected NGINX_CONF_B64 syntax: {b64_entries[0]}"
    decoded = base64.b64decode(m.group(1)).decode("utf-8")
    assert "listen 80;" in decoded
    assert "location = /" in decoded
    assert "proxy_pass http://runtipi-reverse-proxy:80;" in decoded
    assert "location ~ ^/error\\.html/?$" in decoded

    flt_cmd = "\n".join(str(c) for c in flt.get("command", []))
    assert "base64 -d" in flt_cmd
    assert "data:" in flt_cmd
    assert "nginx -g 'daemon off;'" in flt_cmd

    ng = services["ngrok"]
    assert ng.get("image") == f"ngrok/ngrok:{config['version']}-alpine"
    assert ng.get("container_name") == "${CONTAINER_NAME:-ngrok}"
    assert [str(p) for p in ng.get("ports", [])] == ["${APP_PORT}:4040"]
    assert "volumes" not in ng
    assert "healthcheck" in ng
    assert ng.get("x-runtipi", {}).get("is_main") is True
    assert ng.get("x-runtipi", {}).get("internal_port") == 4040

    ng_env = [str(e) for e in ng.get("environment", [])]
    assert "NGROK_URL" in ng_env
    assert "NGROK_TARGET" in ng_env
    assert "ENABLE_NGINX_FILTER=${ENABLE_NGINX_FILTER:-true}" in ng_env
    assert "FILTERS_TARGET=${FILTERS_CONTAINER_NAME:-ngrok-filtros}:80" in ng_env

    ng_cmd = "\n".join(str(c) for c in ng.get("command", []))
    assert "/entrypoint.sh http" in ng_cmd
    assert "--url=" in ng_cmd
    assert 'TARGET="80"' in ng_cmd


def test_ngrok_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "ngrok" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "ngrok logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w == 512
