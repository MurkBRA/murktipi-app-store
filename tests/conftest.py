import os
import json
import yaml
import pytest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
APPS_DIR = REPO_ROOT / "apps"

STANDARD_RUNTIPI_VARS = {
    "APP_DATA_DIR",
    "APP_PORT",
    "TZ",
    "RUNTIPI_APP_DATA_PATH",
}

@pytest.fixture(scope="session")
def repo_root():
    return REPO_ROOT

@pytest.fixture(scope="session")
def apps_dir():
    return APPS_DIR

@pytest.fixture(scope="session")
def app_ids():
    assert APPS_DIR.exists(), f"Apps directory does not exist: {APPS_DIR}"
    dirs = [d.name for d in APPS_DIR.iterdir() if d.is_dir() and not d.name.startswith(".")]
    assert len(dirs) > 0, "No apps found in apps directory"
    return sorted(dirs)

@pytest.fixture
def get_app_config():
    def _loader(app_id):
        config_path = APPS_DIR / app_id / "config.json"
        assert config_path.exists(), f"Missing config.json in {app_id}"
        with open(config_path, "r", encoding="utf-8") as f:
            return json.load(f)
    return _loader

@pytest.fixture
def get_app_compose():
    def _loader(app_id):
        compose_path = APPS_DIR / app_id / "docker-compose.yml"
        assert compose_path.exists(), f"Missing docker-compose.yml in {app_id}"
        with open(compose_path, "r", encoding="utf-8") as f:
            return yaml.safe_load(f)
    return _loader
