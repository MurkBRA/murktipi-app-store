from PIL import Image


def test_telegram_bot_api_config_fields_and_defaults(get_app_config):
    config = get_app_config("telegram-bot-api")
    assert config["id"] == "telegram-bot-api"
    assert config["name"] == "Telegram Bot API"
    assert config["port"] == 2627
    assert config["dynamic_config"] is True
    assert "development" in config["categories"]

    form_fields = config.get("form_fields", [])
    assert len(form_fields) == 7, (
        f"Expected strictly 7 install form fields (6 env vars + optional CONTAINER_NAME, excluding TZ), found {len(form_fields)}"
    )

    fields_by_var = {f["env_variable"]: f for f in form_fields}
    assert "TZ" not in fields_by_var, "TZ must not be exposed as an install form field"

    assert set(fields_by_var.keys()) == {
        "TELEGRAM_API_ID",
        "TELEGRAM_API_HASH",
        "TELEGRAM_LOCAL",
        "TELEGRAM_HTTP_PORT",
        "TELEGRAM_STAT",
        "TELEGRAM_VERBOSITY",
        "CONTAINER_NAME",
    }

    # Mandatory client credentials
    assert fields_by_var["TELEGRAM_API_ID"]["required"] is True
    assert "default" not in fields_by_var["TELEGRAM_API_ID"]

    assert fields_by_var["TELEGRAM_API_HASH"]["required"] is True
    assert fields_by_var["TELEGRAM_API_HASH"]["type"] == "password"
    assert "default" not in fields_by_var["TELEGRAM_API_HASH"]

    # Optional fields keeping original compose defaults untouched
    assert fields_by_var["TELEGRAM_LOCAL"]["required"] is False
    assert fields_by_var["TELEGRAM_LOCAL"]["default"] == "1"

    assert fields_by_var["TELEGRAM_HTTP_PORT"]["required"] is False
    assert fields_by_var["TELEGRAM_HTTP_PORT"]["default"] == 2627

    assert fields_by_var["TELEGRAM_STAT"]["required"] is False
    assert fields_by_var["TELEGRAM_STAT"]["default"] == "1"

    assert fields_by_var["TELEGRAM_VERBOSITY"]["required"] is False
    assert fields_by_var["TELEGRAM_VERBOSITY"]["default"] == 0

    assert fields_by_var["CONTAINER_NAME"]["required"] is False
    assert fields_by_var["CONTAINER_NAME"]["default"] == "telegram-bot-api"


def test_telegram_bot_api_compose_structure(get_app_compose):
    compose = get_app_compose("telegram-bot-api")
    services = compose["services"]

    assert "telegram-bot-api" in services
    svc = services["telegram-bot-api"]

    assert svc.get("image") == "aiogram/telegram-bot-api:latest"
    assert svc.get("container_name") == "${CONTAINER_NAME:-telegram-bot-api}"
    assert svc.get("restart") == "unless-stopped"

    env = svc.get("environment", {})
    assert env.get("TZ") == "${TZ}"
    assert env.get("TELEGRAM_API_ID") == "${TELEGRAM_API_ID}"
    assert env.get("TELEGRAM_API_HASH") == "${TELEGRAM_API_HASH}"
    assert env.get("TELEGRAM_LOCAL") == "${TELEGRAM_LOCAL:-1}"
    assert env.get("TELEGRAM_HTTP_PORT") == "${TELEGRAM_HTTP_PORT:-2627}"
    assert env.get("TELEGRAM_STAT") == "${TELEGRAM_STAT:-1}"
    assert env.get("TELEGRAM_VERBOSITY") == "${TELEGRAM_VERBOSITY:-0}"

    ports = [str(p) for p in svc.get("ports", [])]
    assert "${APP_PORT}:2627" in ports

    volumes = [str(v) for v in svc.get("volumes", [])]
    assert "${APP_DATA_DIR}/data:/var/lib/telegram-bot-api" in volumes

    runtipi_cfg = svc.get("x-runtipi", {})
    assert runtipi_cfg.get("is_main") is True
    assert runtipi_cfg.get("internal_port") == 2627


def test_telegram_bot_api_logo_is_square_jpg(apps_dir):
    logo_path = apps_dir / "telegram-bot-api" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "telegram-bot-api logo.jpg does not exist"
    with Image.open(logo_path) as img:
        assert img.format == "JPEG"
        w, h = img.size
        assert w == h and w >= 512
