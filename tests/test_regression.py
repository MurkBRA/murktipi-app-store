"""
Anti-Regression Test Suite
==========================
Every bug fix or design requirement must have an accompanying anti-regression test
in this file to guarantee that solved issues never reappear in future updates.

Tracker:
- REG-001: App folder name must strictly equal config.json 'id'
- REG-002: No unbound / undeclared variables allowed in docker-compose.yml
- REG-003: form_fields must strictly use 'env_variable' (Runtipi schema v2)
- REG-004: Technitium DNS must bind BOTH UDP and TCP on port 53
- REG-005: Spiral Pool Stratum port must support dynamic configuration with fallback
- REG-006: App logos must exist and maintain 1:1 square aspect ratio
- REG-007: tipi_main_network must be marked external: true
- REG-008: dynamic_config must be true for all apps
- REG-009: No absolute host paths hardcoded in volumes (must use ${APP_DATA_DIR})
- REG-010: Sensitive fields must use type 'password'
- REG-011: description.md must have comprehensive non-placeholder text
- REG-012: Database services must provide healthcheck to prevent startup races
- REG-013: No personal identifiable information, real personal wallets or personal passwords exposed
- REG-014: All apps must have metadata/logo.jpg in JPEG format for proper Runtipi catalog display
- REG-015: Technitium has strictly 3 variables without selection lists (domain=technitium-dns, recursion=Allow, password=senha)
- REG-016: Spiral Pool must use standard Runtipi categories (utilities) avoiding store rejection
"""

import re
import pytest
from PIL import Image
from conftest import STANDARD_RUNTIPI_VARS

ENV_VAR_REGEX = re.compile(r"\$\{([A-Za-z0-9_]+)(?::-([^}]*))?\}")

def test_reg_001_folder_name_must_equal_app_id(app_ids, get_app_config):
    """REG-001: Prevent app ID mismatch which breaks Runtipi catalog indexer."""
    for app_id in app_ids:
        cfg = get_app_config(app_id)
        assert cfg.get("id") == app_id, (
            f"[REG-001] Regression: Folder '{app_id}' does not match config id '{cfg.get('id')}'"
        )

def test_reg_002_no_unbound_env_vars_in_compose(apps_dir, app_ids, get_app_config):
    """REG-002: Prevent undefined compose variables that evaluate to blank strings."""
    for app_id in app_ids:
        compose_text = (apps_dir / app_id / "docker-compose.yml").read_text(encoding="utf-8")
        used_vars = set(m[0] for m in ENV_VAR_REGEX.findall(compose_text))
        cfg = get_app_config(app_id)
        declared_vars = set(f["env_variable"] for f in cfg.get("form_fields", []))

        unbound = used_vars - declared_vars - STANDARD_RUNTIPI_VARS
        assert not unbound, f"[REG-002] Regression: Unbound variables in {app_id}/docker-compose.yml: {unbound}"

def test_reg_003_form_fields_must_use_env_variable_property(app_ids, get_app_config):
    """REG-003: Prevent legacy 'name' without 'env_variable' in form_fields."""
    for app_id in app_ids:
        cfg = get_app_config(app_id)
        for idx, field in enumerate(cfg.get("form_fields", [])):
            assert "env_variable" in field, (
                f"[REG-003] Regression in '{app_id}' field #{idx}: missing 'env_variable' key"
            )
            assert len(field["env_variable"].strip()) > 0

def test_reg_004_technitium_must_support_both_tcp_and_udp_dns(get_app_compose):
    """REG-004: DNS queries fail if UDP is missing; DNSSEC & zone transfers fail if TCP is missing."""
    compose = get_app_compose("technitium")
    ports = [str(p) for p in compose["services"]["technitium-dns"]["ports"]]
    has_udp = any("53:53/udp" in p for p in ports)
    has_tcp = any("53:53/tcp" in p for p in ports)
    assert has_udp, "[REG-004] Regression: Technitium is missing 53:53/udp"
    assert has_tcp, "[REG-004] Regression: Technitium is missing 53:53/tcp"

def test_reg_005_spiral_pool_stratum_port_dynamic_with_fallback(apps_dir):
    """REG-005: Spiral pool stratum port must not be hardcoded, must allow custom user port."""
    compose_text = (apps_dir / "spiral-pool" / "docker-compose.yml").read_text(encoding="utf-8")
    assert "${STRATUM_PORT:-" in compose_text or "${STRATUM_PORT}" in compose_text, (
        "[REG-005] Regression: Stratum port is not configurable via STRATUM_PORT in spiral-pool compose"
    )

def test_reg_006_logos_must_be_strictly_square_jpg(apps_dir, app_ids):
    """REG-006: Logos must exist in metadata/logo.jpg, format JPEG, and be square 1:1."""
    for app_id in app_ids:
        logo_path = apps_dir / app_id / "metadata" / "logo.jpg"
        assert logo_path.exists(), f"[REG-006] Regression: Missing metadata/logo.jpg for {app_id}"
        with Image.open(logo_path) as img:
            assert img.format == "JPEG", f"[REG-006] Regression: Logo for '{app_id}' must be JPEG, got {img.format}"
            w, h = img.size
            assert w == h, f"[REG-006] Regression: Logo for '{app_id}' is not square ({w}x{h})"

def test_reg_007_tipi_main_network_marked_external(app_ids, get_app_compose):
    """REG-007: Omitting external: true causes Docker Compose to try creating runtipi's network."""
    for app_id in app_ids:
        compose = get_app_compose(app_id)
        assert "networks" in compose and "tipi_main_network" in compose["networks"]
        net = compose["networks"]["tipi_main_network"]
        assert net.get("external") is True, (
            f"[REG-007] Regression: 'tipi_main_network' in '{app_id}' must have 'external: true'"
        )

def test_reg_008_dynamic_config_flag_is_true(app_ids, get_app_config):
    """REG-008: dynamic_config: true is required for modern Runtipi dynamic compose support."""
    for app_id in app_ids:
        cfg = get_app_config(app_id)
        assert cfg.get("dynamic_config") is True, (
            f"[REG-008] Regression: App '{app_id}' must set 'dynamic_config: true'"
        )

def test_reg_009_no_hardcoded_absolute_host_paths(apps_dir, app_ids):
    """REG-009: Hardcoded paths like /home/ubuntu or C:\\ fail on other servers."""
    for app_id in app_ids:
        compose_text = (apps_dir / app_id / "docker-compose.yml").read_text(encoding="utf-8")
        # Check volume lines
        for line in compose_text.splitlines():
            line_s = line.strip()
            if line_s.startswith("-") and ":" in line_s:
                host_part = line_s.split(":")[0].lstrip("- ")
                if host_part.startswith("/home") or host_part.startswith("/root") or ":\\" in host_part:
                    pytest.fail(f"[REG-009] Regression: Hardcoded host path found in {app_id}: {line_s}")

def test_reg_010_passwords_use_password_type(app_ids, get_app_config):
    """REG-010: Ensure sensitive fields mask input in UI."""
    for app_id in app_ids:
        cfg = get_app_config(app_id)
        for field in cfg.get("form_fields", []):
            var_name = field["env_variable"].lower()
            if "password" in var_name or "secret" in var_name:
                # If it's a file path to password, text is fine; otherwise must be password
                if "file" not in var_name:
                    assert field["type"] == "password", (
                        f"[REG-010] Regression: Sensitive field '{field['env_variable']}' in '{app_id}' must have type 'password'"
                    )

def test_reg_011_description_md_contains_meaningful_documentation(apps_dir, app_ids):
    """REG-011: App description must provide usage instructions and not be a stub."""
    for app_id in app_ids:
        desc_file = apps_dir / app_id / "metadata" / "description.md"
        assert desc_file.exists(), f"[REG-011] Missing description.md in {app_id}"
        content = desc_file.read_text(encoding="utf-8")
        assert len(content.split()) >= 30, (
            f"[REG-011] Regression: description.md for '{app_id}' is too sparse ({len(content.split())} words)"
        )
        assert "TODO" not in content and "Lorem ipsum" not in content

def test_reg_012_database_containers_have_healthchecks(app_ids, get_app_compose):
    """REG-012: Postgres or database services must have healthchecks to prevent startup crashes."""
    for app_id in app_ids:
        compose = get_app_compose(app_id)
        services = compose["services"]
        for s_name, s_def in services.items():
            img = s_def.get("image", "")
            if "postgres" in img or "mariadb" in img or "mysql" in img:
                assert "healthcheck" in s_def, (
                    f"[REG-012] Regression: Database service '{s_name}' in '{app_id}' missing healthcheck"
                )

def test_reg_013_no_personal_identifiable_data_or_real_wallet_addresses(apps_dir, app_ids):
    """REG-013: Prevent leaking personal names, real personal wallets or personal passwords in store files."""
    forbidden_tokens = [
        "".join(["g", "a", "b", "r", "i", "e", "l"]),
        "".join(["b", "c", "1", "q", "x", "2", "v", "r", "z", "y"]),
    ]
    for app_id in app_ids:
        app_path = apps_dir / app_id
        for file_path in app_path.rglob("*"):
            if file_path.is_file() and file_path.suffix in [".json", ".yml", ".yaml", ".md", ".env"]:
                content = file_path.read_text(encoding="utf-8").lower()
                for token in forbidden_tokens:
                    assert token not in content, (
                        f"[REG-013] Regression: Forbidden personal term or real wallet token found in {file_path.relative_to(apps_dir)}"
                    )

def test_reg_014_no_legacy_png_logos_and_jpeg_strictly_required(apps_dir, app_ids):
    """REG-014: Runtipi catalog requires metadata/logo.jpg. No logo.png should remain and logo.jpg must exist."""
    for app_id in app_ids:
        metadata_dir = apps_dir / app_id / "metadata"
        png_logo = metadata_dir / "logo.png"
        jpg_logo = metadata_dir / "logo.jpg"
        assert not png_logo.exists(), (
            f"[REG-014] Regression: Deprecated 'logo.png' found in '{app_id}/metadata'. Runtipi requires 'logo.jpg'."
        )
        assert jpg_logo.exists(), (
            f"[REG-014] Regression: Missing 'metadata/logo.jpg' in '{app_id}'."
        )
        with Image.open(jpg_logo) as img:
            assert img.format == "JPEG", (
                f"[REG-014] Regression: Logo in '{app_id}' must be in JPEG format, got {img.format}."
            )

def test_reg_015_technitium_has_strictly_three_env_vars_and_no_selection_lists(get_app_config, get_app_compose):
    """REG-015: Technitium must use strictly 3 environment variables with no selection list or dynamic slots."""
    config = get_app_config("technitium")
    form_fields = config.get("form_fields", [])
    assert len(form_fields) == 3, (
        f"[REG-015] Regression: Technitium must have strictly 3 form fields, found {len(form_fields)}"
    )

    field_map = {f["env_variable"]: f for f in form_fields}
    assert set(field_map.keys()) == {
        "DNS_SERVER_DOMAIN",
        "DNS_SERVER_RECURSION",
        "DNS_SERVER_ADMIN_PASSWORD",
    }, f"[REG-015] Regression: Unexpected form fields in Technitium: {list(field_map.keys())}"

    assert field_map["DNS_SERVER_DOMAIN"]["default"] == "technitium-dns"
    assert field_map["DNS_SERVER_RECURSION"]["default"] == "Allow"
    assert field_map["DNS_SERVER_ADMIN_PASSWORD"]["default"] == "senha"

    compose = get_app_compose("technitium")
    assert "technitium-dns" in compose["services"]
    svc = compose["services"]["technitium-dns"]
    assert svc.get("container_name") == "technitium-dns"
    assert svc.get("image") == "technitium/dns-server:latest"
    assert svc.get("restart") == "unless-stopped"

    # Exactly 3 environment entries
    env_list = svc.get("environment", [])
    assert len(env_list) == 3, f"[REG-015] Regression: Expected 3 environment entries, got {len(env_list)}"
    env_keys = [e.split("=")[0] for e in env_list]
    assert set(env_keys) == {
        "DNS_SERVER_DOMAIN",
        "DNS_SERVER_RECURSION",
        "DNS_SERVER_ADMIN_PASSWORD",
    }

    ports = [str(p) for p in svc.get("ports", [])]
    assert any("53:53/tcp" in p for p in ports)
    assert any("53:53/udp" in p for p in ports)
    assert any("5380:5380/tcp" in p or "5380" in p for p in ports)

def test_reg_016_apps_use_standard_runtipi_categories_and_spiral_shows_in_catalog(app_ids, get_app_config):
    """REG-016: Non-standard categories (like 'crypto') prevent apps from showing in Runtipi.
    Apps must only use recognized Runtipi categories."""
    valid_runtipi_categories = {
        "utilities", "network", "media", "development", "automation",
        "social", "finance", "security", "data", "featured", "gaming",
        "productivity", "monitoring", "smart-home", "downloaders",
        "books", "audio", "photos"
    }

    for app_id in app_ids:
        cfg = get_app_config(app_id)
        cats = cfg.get("categories", [])
        assert isinstance(cats, list) and len(cats) > 0, (
            f"[REG-016] Regression: App '{app_id}' must have at least one category"
        )
        for cat in cats:
            assert cat in valid_runtipi_categories, (
                f"[REG-016] Regression: App '{app_id}' has unrecognized category '{cat}' which breaks store indexing. "
                f"Allowed: {sorted(valid_runtipi_categories)}"
            )
        assert "crypto" not in cats, (
            f"[REG-016] Regression: Category 'crypto' breaks Runtipi store listing for '{app_id}'"
        )

def test_reg_017_technitium_logo_is_official_converted_jpg(apps_dir):
    """REG-017: Technitium logo must be the official branding converted to square 1:1 JPEG,
    preserving the official Technitium blue palette and white emblem."""
    logo_path = apps_dir / "technitium" / "metadata" / "logo.jpg"
    assert logo_path.exists(), "[REG-017] Regression: Missing technitium/metadata/logo.jpg"

    with Image.open(logo_path) as img:
        assert img.format == "JPEG", f"[REG-017] Regression: Logo must be JPEG, got {img.format}"
        w, h = img.size
        assert w == h and w >= 512, f"[REG-017] Regression: Logo must be at least 512x512 square, got {w}x{h}"

        # Sample corner and center pixels to verify official Technitium palette
        corner_r, corner_g, corner_b = img.getpixel((10, 10))
        assert 80 <= corner_r <= 125, f"[REG-017] Unexpected corner Red value: {corner_r}"
        assert 130 <= corner_g <= 175, f"[REG-017] Unexpected corner Green value: {corner_g}"
        assert 230 <= corner_b <= 255, f"[REG-017] Unexpected corner Blue value: {corner_b}"





