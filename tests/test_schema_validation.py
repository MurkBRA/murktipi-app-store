import json
from pathlib import Path
from PIL import Image

REQUIRED_CONFIG_FIELDS = [
    "name",
    "id",
    "available",
    "short_desc",
    "description",
    "author",
    "port",
    "categories",
    "tipi_version",
    "version",
    "source",
    "exposable",
]

ALLOWED_FIELD_TYPES = {
    "text",
    "password",
    "email",
    "number",
    "fqdn",
    "ip",
    "fqdnip",
    "random",
    "boolean",
}

def test_all_apps_have_required_structure(apps_dir, app_ids):
    for app_id in app_ids:
        app_path = apps_dir / app_id
        config_path = app_path / "config.json"
        compose_path = app_path / "docker-compose.yml"
        metadata_path = app_path / "metadata"
        desc_path = metadata_path / "description.md"

        assert config_path.exists(), f"App '{app_id}' is missing config.json"
        assert compose_path.exists(), f"App '{app_id}' is missing docker-compose.yml"
        assert metadata_path.exists(), f"App '{app_id}' is missing metadata folder"
        assert desc_path.exists(), f"App '{app_id}' is missing metadata/description.md"
        assert desc_path.stat().st_size > 50, f"App '{app_id}' description.md is too short or empty"

        # Check logo
        logo_png = metadata_path / "logo.png"
        logo_jpg = metadata_path / "logo.jpg"
        assert logo_png.exists() or logo_jpg.exists(), f"App '{app_id}' is missing logo (logo.png or logo.jpg)"

def test_config_json_fields_and_types(app_ids, get_app_config):
    for app_id in app_ids:
        config = get_app_config(app_id)

        # ID must match directory name exactly
        assert config.get("id") == app_id, f"config.json 'id' ({config.get('id')}) does not match folder '{app_id}'"

        # Check all required fields exist
        for field in REQUIRED_CONFIG_FIELDS:
            assert field in config, f"App '{app_id}' config.json missing required field: '{field}'"

        assert isinstance(config["name"], str) and len(config["name"]) > 0
        assert isinstance(config["available"], bool)
        assert isinstance(config["short_desc"], str) and len(config["short_desc"]) > 0
        assert isinstance(config["description"], str) and len(config["description"]) > 0
        assert isinstance(config["author"], str) and len(config["author"]) > 0
        assert isinstance(config["port"], int) and 1 <= config["port"] <= 65535
        assert isinstance(config["categories"], list) and len(config["categories"]) > 0
        assert isinstance(config["tipi_version"], int) and config["tipi_version"] >= 1
        assert isinstance(config["version"], str) and len(config["version"]) > 0
        assert isinstance(config["source"], str) and config["source"].startswith("http")
        assert isinstance(config["exposable"], bool)

def test_form_fields_structure(app_ids, get_app_config):
    for app_id in app_ids:
        config = get_app_config(app_id)
        form_fields = config.get("form_fields", [])
        assert isinstance(form_fields, list), f"App '{app_id}' form_fields must be a list"

        seen_vars = set()
        for idx, field in enumerate(form_fields):
            assert "type" in field, f"Field #{idx} in '{app_id}' missing 'type'"
            assert field["type"] in ALLOWED_FIELD_TYPES, (
                f"Field #{idx} in '{app_id}' has invalid type '{field['type']}'. Allowed: {ALLOWED_FIELD_TYPES}"
            )
            assert "label" in field and len(field["label"]) > 0, f"Field #{idx} in '{app_id}' missing 'label'"
            assert "env_variable" in field and len(field["env_variable"]) > 0, (
                f"Field #{idx} in '{app_id}' missing 'env_variable'"
            )

            var_name = field["env_variable"]
            assert var_name not in seen_vars, f"Duplicate env_variable '{var_name}' in '{app_id}' form_fields"
            seen_vars.add(var_name)

            # If options are present, check their structure
            if "options" in field:
                assert isinstance(field["options"], list) and len(field["options"]) > 0
                for opt in field["options"]:
                    assert "label" in opt and "value" in opt

def test_logo_validity_and_aspect_ratio(apps_dir, app_ids):
    for app_id in app_ids:
        metadata_path = apps_dir / app_id / "metadata"
        logo_path = metadata_path / "logo.png"
        if not logo_path.exists():
            logo_path = metadata_path / "logo.jpg"

        with Image.open(logo_path) as img:
            w, h = img.size
            assert w == h, f"App '{app_id}' logo must be square (1:1 aspect ratio), but got {w}x{h}"
            assert w >= 128, f"App '{app_id}' logo resolution too small ({w}x{h}), expected >= 128x128"
