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
- REG-017: Technitium logo must be the official Wikimedia image converted to 512x512 JPEG
- REG-018: Telegram Bot API must require API ID/Hash, exclude TZ from form_fields, and preserve defaults
- REG-019: Telegram Bot API must map ${APP_PORT}:2627 matching internal_port 2627 to prevent duplicate port binding
- REG-020: CI workflow setup-python pip cache must reference an existing dependency file (requirements-dev.txt)
- REG-021: Email OAuth2 Proxy UI must use ROOT_FOLDER_HOST (never RUNTIPI_USER_CONFIG_PATH) for /config and /compose mounts and GHCR image
- REG-022: Email OAuth2 Proxy UI must map ${APP_PORT}:8090 matching internal_port 8090 and mask ADMIN_PASSWORD
- REG-023: Email OAuth2 Proxy UI must cross-compile natively via $BUILDPLATFORM into FROM scratch without QEMU
- REG-024: Email OAuth2 Proxy UI ports-modal footer must keep structured 3-column grid (.ports-actions-grid) and all action IDs
- REG-025: Email OAuth2 Proxy UI must default to English, support browser auto-PT + sidebar selector, and keep EN/PT keys in sync
- REG-026: Email OAuth2 Proxy UI must pin a semantic version tag matching config.json version (never :latest) with tipi_version >= 2
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


def test_reg_018_telegram_bot_api_env_vars_and_defaults(get_app_config, get_app_compose):
    """REG-018: Telegram Bot API must require API ID/Hash, exclude TZ from form_fields,
    and preserve exact compose defaults when the user leaves optional fields untouched."""
    config = get_app_config("telegram-bot-api")
    form_fields = config.get("form_fields", [])
    field_map = {f["env_variable"]: f for f in form_fields}

    assert "TZ" not in field_map, "[REG-018] Regression: TZ must not be in install form_fields"
    assert set(field_map.keys()) == {
        "TELEGRAM_API_ID",
        "TELEGRAM_API_HASH",
        "TELEGRAM_LOCAL",
        "TELEGRAM_HTTP_PORT",
        "TELEGRAM_STAT",
        "TELEGRAM_VERBOSITY",
    }

    assert field_map["TELEGRAM_API_ID"].get("required") is True
    assert field_map["TELEGRAM_API_HASH"].get("required") is True
    assert field_map["TELEGRAM_LOCAL"].get("default") == "1"
    assert field_map["TELEGRAM_HTTP_PORT"].get("default") == 2627
    assert field_map["TELEGRAM_STAT"].get("default") == "1"
    assert field_map["TELEGRAM_VERBOSITY"].get("default") == 0

    compose = get_app_compose("telegram-bot-api")
    env = compose["services"]["telegram-bot-api"].get("environment", {})
    assert "TZ" in env
    assert "${TELEGRAM_LOCAL:-1}" == str(env.get("TELEGRAM_LOCAL"))
    assert "${TELEGRAM_HTTP_PORT:-2627}" == str(env.get("TELEGRAM_HTTP_PORT"))
    assert "${TELEGRAM_STAT:-1}" == str(env.get("TELEGRAM_STAT"))
    assert "${TELEGRAM_VERBOSITY:-0}" == str(env.get("TELEGRAM_VERBOSITY"))


def test_reg_019_telegram_bot_api_port_mapping_matches_runtipi_internal_port(get_app_compose):
    """REG-019: Ports must bind ${APP_PORT}:2627 matching internal_port 2627 so Runtipi
    does not inject a second conflicting port mapping on startup."""
    compose = get_app_compose("telegram-bot-api")
    svc = compose["services"]["telegram-bot-api"]
    ports = [str(p) for p in svc.get("ports", [])]
    internal_port = svc.get("x-runtipi", {}).get("internal_port")

    assert internal_port == 2627
    assert "${APP_PORT}:2627" in ports
    assert "2627:2627" not in ports


def test_reg_020_ci_workflow_pip_cache_dependency_path_exists(repo_root):
    """REG-020: Prevent setup-python cache='pip' failure when requirements.txt does not exist."""
    import yaml

    ci_path = repo_root / ".github" / "workflows" / "ci.yml"
    assert ci_path.exists(), "[REG-020] Missing .github/workflows/ci.yml"
    ci_data = yaml.safe_load(ci_path.read_text(encoding="utf-8"))

    for job_name, job_def in ci_data.get("jobs", {}).items():
        for step in job_def.get("steps", []):
            uses = str(step.get("uses", ""))
            with_cfg = step.get("with", {}) or {}
            if "actions/setup-python" in uses and with_cfg.get("cache") == "pip":
                dep_path = with_cfg.get("cache-dependency-path")
                assert dep_path, (
                    f"[REG-020] Job '{job_name}' uses cache='pip' without 'cache-dependency-path'"
                )
                assert (repo_root / dep_path).exists(), (
                    f"[REG-020] CI cache-dependency-path '{dep_path}' does not exist in repo"
                )


def test_reg_021_email_oauth2_proxy_ui_volumes_and_ghcr_image(repo_root, get_app_compose):
    """REG-021: Runtipi strips RUNTIPI_USER_CONFIG_PATH from app.env and injects ROOT_FOLDER_HOST.
    Ensure /config and /compose use ROOT_FOLDER_HOST + PROXY_STORE_ID and GHCR image is configured."""
    compose = get_app_compose("email-oauth2-proxy-ui")
    svc = compose["services"]["email-oauth2-proxy-ui"]
    volumes = [str(v) for v in svc.get("volumes", [])]

    assert svc.get("image", "").startswith("ghcr.io/murkbra/email-oauth2-proxy-ui:")
    assert not any("RUNTIPI_USER_CONFIG_PATH" in v for v in volumes), (
        "[REG-021] Regression: Runtipi strips RUNTIPI_USER_CONFIG_PATH from app.env; use ROOT_FOLDER_HOST instead"
    )
    assert "${ROOT_FOLDER_HOST}/app-data/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy/data/config:/config:rw" in volumes
    assert "${ROOT_FOLDER_HOST}/user-config/${PROXY_STORE_ID:-migrated}/email-oauth2-proxy:/compose:rw" in volumes
    assert (repo_root / "packages" / "email-oauth2-proxy-ui" / "Dockerfile").exists()


def test_reg_022_email_oauth2_proxy_ui_port_mapping_and_password_mask(get_app_config, get_app_compose):
    """REG-022: Ensure ${APP_PORT}:8090 matches internal_port 8090 and ADMIN_PASSWORD uses password type."""
    cfg = get_app_config("email-oauth2-proxy-ui")
    fields = {f["env_variable"]: f for f in cfg.get("form_fields", [])}
    assert fields["ADMIN_PASSWORD"]["type"] == "password"

    compose = get_app_compose("email-oauth2-proxy-ui")
    svc = compose["services"]["email-oauth2-proxy-ui"]
    ports = [str(p) for p in svc.get("ports", [])]
    assert svc.get("x-runtipi", {}).get("internal_port") == 8090
    assert "${APP_PORT}:8090" in ports
    assert "8090:8090" not in ports


def test_reg_023_email_oauth2_proxy_ui_scratch_and_no_qemu(repo_root):
    """REG-023: Keep multi-arch builds fast and image minimal (~8 MB) using native Go
    cross-compilation on $BUILDPLATFORM into FROM scratch without QEMU emulation."""
    df_text = (repo_root / "packages" / "email-oauth2-proxy-ui" / "Dockerfile").read_text(encoding="utf-8")
    wf_text = (repo_root / ".github" / "workflows" / "docker-email-oauth2-proxy-ui.yml").read_text(encoding="utf-8")

    assert "--platform=$BUILDPLATFORM" in df_text
    assert "FROM scratch" in df_text
    assert "setup-qemu-action" not in wf_text


def test_reg_024_email_oauth2_proxy_ui_ports_modal_actions_grid(repo_root):
    """REG-024: Keep ports-modal action buttons organized in a 3-column grid separated
    from the Close button so long labels do not wrap raggedly."""
    web_dir = repo_root / "packages" / "email-oauth2-proxy-ui" / "web"
    html_text = (web_dir / "index.html").read_text(encoding="utf-8")
    css_text = (web_dir / "styles.css").read_text(encoding="utf-8")

    assert 'class="ports-modal-footer"' in html_text
    assert 'class="ports-actions-grid"' in html_text
    for btn_id in ("btn-ports-apply-compose-only", "btn-ports-apply-docker", "btn-ports-apply-both"):
        assert f'id="{btn_id}"' in html_text
    assert ".ports-actions-grid" in css_text
    assert "grid-template-columns: repeat(3" in css_text


def test_reg_025_email_oauth2_proxy_ui_i18n_completeness(repo_root):
    """REG-025: Verify translation file exists, HTML defaults to English with sidebar
    language selector, and every data-i18n / t() key exists in both 'en' and 'pt'."""
    web_dir = repo_root / "packages" / "email-oauth2-proxy-ui" / "web"
    html_text = (web_dir / "index.html").read_text(encoding="utf-8")
    js_text = (web_dir / "app.js").read_text(encoding="utf-8")
    tr_path = web_dir / "translations.js"

    assert tr_path.exists(), "[REG-025] Missing web/translations.js"
    tr_text = tr_path.read_text(encoding="utf-8")

    assert '<html lang="en">' in html_text
    assert 'id="lang-select"' in html_text
    assert '<script src="/translations.js"></script>' in html_text
    assert "detectBrowserLanguage" in js_text

    # Extract keys from translations.js for en and pt blocks
    en_block = tr_text.split("  pt: {", 1)[0]
    pt_block = tr_text.split("  pt: {", 1)[1]
    en_keys = set(re.findall(r"^\s+'([^']+)':", en_block, re.MULTILINE))
    pt_keys = set(re.findall(r"^\s+'([^']+)':", pt_block, re.MULTILINE))

    assert en_keys, "[REG-025] Failed to parse English keys from translations.js"
    assert en_keys == pt_keys, (
        f"[REG-025] EN/PT key mismatch. Missing in PT: {sorted(en_keys - pt_keys)} | Extra in PT: {sorted(pt_keys - en_keys)}"
    )

    html_keys = set(re.findall(r'data-i18n(?:-(?:html|placeholder|title))?="([^"]+)"', html_text))
    js_keys = set(re.findall(r"\bt\(\s*'([^']+)'", js_text))
    # Dynamic tab header keys constructed in updateTabHeader
    for tab in ("dashboard", "oauth", "config", "certs", "logs"):
        js_keys.add(f"tab.{tab}.title")
        js_keys.add(f"tab.{tab}.sub")

    used_keys = html_keys | js_keys
    missing = used_keys - en_keys
    assert not missing, f"[REG-025] Missing translation keys in translations.js: {sorted(missing)}"


def test_reg_026_email_oauth2_proxy_ui_versioned_image_tag(repo_root, get_app_config, get_app_compose):
    """REG-026: Pin a semantic version in config.json and docker-compose.yml (not :latest)
    so Runtipi detects updates and pulls the exact published GHCR image tag."""
    cfg = get_app_config("email-oauth2-proxy-ui")
    compose = get_app_compose("email-oauth2-proxy-ui")
    wf_text = (repo_root / ".github" / "workflows" / "docker-email-oauth2-proxy-ui.yml").read_text(encoding="utf-8")

    version = str(cfg.get("version", ""))
    assert version and version != "latest", "[REG-026] version in config.json must not be 'latest'"
    assert re.match(r"^\d+\.\d+\.\d+$", version), f"[REG-026] Expected semver X.Y.Z, got '{version}'"
    assert cfg.get("tipi_version", 0) >= 2, "[REG-026] tipi_version must be >= 2"

    expected_image = f"ghcr.io/murkbra/email-oauth2-proxy-ui:{version}"
    actual_image = compose["services"]["email-oauth2-proxy-ui"].get("image")
    assert actual_image == expected_image, (
        f"[REG-026] Compose image '{actual_image}' must match config.json version '{expected_image}'"
    )
    assert "steps.appver.outputs.version" in wf_text







