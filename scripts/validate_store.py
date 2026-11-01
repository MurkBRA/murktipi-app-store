#!/usr/bin/env python3
"""
Runtipi App Store Standalone Validator
======================================
Validates all apps in the apps/ directory against Runtipi v2 schema standards,
YAML syntax, environment variable bindings, and logo specifications.
"""

import sys
import json
import re
from pathlib import Path

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

try:
    import yaml
except ImportError:
    print("Error: PyYAML is required. Run: pip install pyyaml")
    sys.exit(1)

try:
    from PIL import Image
except ImportError:
    Image = None

ROOT_DIR = Path(__file__).resolve().parent.parent
APPS_DIR = ROOT_DIR / "apps"

STANDARD_RUNTIPI_VARS = {
    "APP_DATA_DIR",
    "APP_PORT",
    "TZ",
    "RUNTIPI_APP_DATA_PATH",
}

ENV_VAR_REGEX = re.compile(r"\$\{([A-Za-z0-9_]+)(?::-([^}]*))?\}")

def log_success(msg):
    print(f"  \033[32m✔\033[0m {msg}")

def log_error(msg):
    print(f"  \033[31m✖\033[0m {msg}")

def validate_app(app_path: Path) -> list:
    errors = []
    app_id = app_path.name
    print(f"\n\033[1;34m[Validating App: {app_id}]\033[0m")

    # 1. Check required files
    config_file = app_path / "config.json"
    compose_file = app_path / "docker-compose.yml"
    metadata_dir = app_path / "metadata"
    desc_file = metadata_dir / "description.md"
    logo_file = metadata_dir / "logo.jpg"

    if not config_file.exists():
        errors.append("Missing config.json")
    if not compose_file.exists():
        errors.append("Missing docker-compose.yml")
    if not desc_file.exists():
        errors.append("Missing metadata/description.md")
    if not logo_file.exists():
        errors.append("Missing metadata/logo.jpg (required format: .jpg)")

    if errors:
        for err in errors:
            log_error(err)
        return errors

    # 2. Validate config.json
    try:
        with open(config_file, "r", encoding="utf-8") as f:
            config = json.load(f)
    except Exception as e:
        errors.append(f"Invalid JSON in config.json: {e}")
        return errors

    if config.get("id") != app_id:
        errors.append(f"config.json 'id' ({config.get('id')}) does not match folder name ({app_id})")

    req_fields = ["name", "id", "available", "short_desc", "description", "author", "port", "categories", "tipi_version", "version", "source", "exposable"]
    for field in req_fields:
        if field not in config:
            errors.append(f"Missing required config field: '{field}'")

    declared_vars = set()
    for idx, field in enumerate(config.get("form_fields", [])):
        if "env_variable" not in field:
            errors.append(f"form_fields item #{idx} missing 'env_variable'")
        else:
            declared_vars.add(field["env_variable"])

    if not errors:
        log_success("config.json schema & fields are valid")

    # 3. Validate docker-compose.yml
    try:
        with open(compose_file, "r", encoding="utf-8") as f:
            raw_compose = f.read()
            compose = yaml.safe_load(raw_compose)
    except Exception as e:
        errors.append(f"Invalid YAML in docker-compose.yml: {e}")
        return errors

    if not isinstance(compose, dict):
        errors.append("docker-compose.yml must parse to a dictionary")
        return errors

    if compose.get("x-runtipi", {}).get("schema_version") != 2:
        errors.append("docker-compose.yml must have x-runtipi.schema_version: 2")

    services = compose.get("services", {})
    main_found = False
    for s_name, s_def in services.items():
        if isinstance(s_def, dict) and s_def.get("x-runtipi", {}).get("is_main") is True:
            main_found = True
            break
    if not main_found:
        errors.append("No service has 'x-runtipi.is_main: true' configured")

    # Check unbound vars
    referenced_vars = set(m[0] for m in ENV_VAR_REGEX.findall(raw_compose))
    unbound = referenced_vars - declared_vars - STANDARD_RUNTIPI_VARS
    if unbound:
        errors.append(f"Unbound variables in docker-compose.yml: {unbound}")

    if not any("docker-compose" in e or "Unbound" in e or "is_main" in e for e in errors):
        log_success("docker-compose.yml syntax, schema v2 and variables are valid")

    # 4. Validate logo
    if Image and logo_file.exists():
        try:
            with Image.open(logo_file) as img:
                w, h = img.size
                if w != h:
                    errors.append(f"Logo is not square (1:1 ratio), dimensions: {w}x{h}")
                else:
                    log_success(f"Logo is valid square image ({w}x{h})")
        except Exception as e:
            errors.append(f"Failed to read logo image: {e}")

    # 5. Validate description.md
    if desc_file.exists():
        content = desc_file.read_text(encoding="utf-8")
        if len(content.strip()) < 50:
            errors.append("metadata/description.md is too short")
        else:
            log_success(f"metadata/description.md verified ({len(content.split())} words)")

    for err in errors:
        log_error(err)

    return errors

def main():
    print("\033[1;36m=== Murktipi App Store Validator ===\033[0m")
    if not APPS_DIR.exists():
        print(f"Error: Apps directory not found at {APPS_DIR}")
        sys.exit(1)

    all_errors = {}
    apps = [d for d in APPS_DIR.iterdir() if d.is_dir() and not d.name.startswith(".")]

    if not apps:
        print("Warning: No apps found in apps/")
        sys.exit(1)

    for app in sorted(apps, key=lambda p: p.name):
        errs = validate_app(app)
        if errs:
            all_errors[app.name] = errs

    print("\n" + "=" * 45)
    if all_errors:
        print(f"\033[1;31mValidation failed with errors in {len(all_errors)} app(s).\033[0m")
        sys.exit(1)
    else:
        print(f"\033[1;32mAll {len(apps)} app(s) passed validation successfully!\033[0m")
        sys.exit(0)

if __name__ == "__main__":
    main()
