import re
import yaml
from pathlib import Path
from conftest import STANDARD_RUNTIPI_VARS

ENV_VAR_REGEX = re.compile(r"\$\{([A-Za-z0-9_]+)(?::-([^}]*))?\}")

def test_compose_yaml_syntax_and_schema_version(app_ids, get_app_compose):
    for app_id in app_ids:
        compose = get_app_compose(app_id)
        assert isinstance(compose, dict), f"Compose for '{app_id}' must parse to a dictionary"

        # Check x-runtipi schema version 2
        assert "x-runtipi" in compose, f"'{app_id}' docker-compose.yml must have root 'x-runtipi' declaration"
        assert compose["x-runtipi"].get("schema_version") == 2, (
            f"'{app_id}' docker-compose.yml must specify 'x-runtipi.schema_version: 2'"
        )

        # Check services exist
        assert "services" in compose and len(compose["services"]) > 0, f"No services defined for '{app_id}'"

def test_main_service_declaration(app_ids, get_app_compose):
    for app_id in app_ids:
        compose = get_app_compose(app_id)
        services = compose["services"]

        main_services = []
        for s_name, s_def in services.items():
            if isinstance(s_def, dict) and "x-runtipi" in s_def:
                runtipi_cfg = s_def["x-runtipi"]
                if runtipi_cfg.get("is_main") is True:
                    main_services.append((s_name, runtipi_cfg))

        assert len(main_services) == 1, (
            f"App '{app_id}' must have exactly ONE service with 'x-runtipi.is_main: true', found {len(main_services)}"
        )
        _, main_cfg = main_services[0]
        assert "internal_port" in main_cfg, f"Main service in '{app_id}' must specify 'internal_port'"
        assert isinstance(main_cfg["internal_port"], int) and 1 <= main_cfg["internal_port"] <= 65535

def test_all_compose_env_vars_are_declared(apps_dir, app_ids, get_app_config):
    for app_id in app_ids:
        compose_path = apps_dir / app_id / "docker-compose.yml"
        raw_text = compose_path.read_text(encoding="utf-8")

        # Extract all ${VAR} or ${VAR:-default}
        referenced_vars = set(m[0] for m in ENV_VAR_REGEX.findall(raw_text))

        # Config declared variables
        config = get_app_config(app_id)
        form_fields = config.get("form_fields", [])
        declared_vars = set(f["env_variable"] for f in form_fields)

        # All referenced vars must be in form_fields or standard runtipi vars
        unbound = referenced_vars - declared_vars - STANDARD_RUNTIPI_VARS
        assert not unbound, (
            f"App '{app_id}' references environment variables in compose not declared in config.json or standard Runtipi: {unbound}"
        )

def test_network_tipi_main_network_is_external(app_ids, get_app_compose):
    for app_id in app_ids:
        compose = get_app_compose(app_id)
        assert "networks" in compose, f"App '{app_id}' missing networks section"
        assert "tipi_main_network" in compose["networks"], f"App '{app_id}' missing 'tipi_main_network'"
        net = compose["networks"]["tipi_main_network"]
        assert net.get("external") is True, f"'tipi_main_network' in '{app_id}' must be marked 'external: true'"
