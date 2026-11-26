import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import pytest
from config_builder import (
    build_camoufox_kwargs,
    parse_bool_env,
    parse_headless_env,
    parse_viewer_enabled,
    parse_viewer_resolution,
)


def test_default_camoufox_kwargs():
    kwargs = build_camoufox_kwargs({})
    assert kwargs == {
        "host": "127.0.0.1",
        "port": 9224,
        "ws_path": "/camoufox",
        "headless": "virtual",
    }


def test_optional_flags_and_proxy_mapping():
    kwargs = build_camoufox_kwargs({
        "HEADLESS": "true",
        "GEOIP": "true",
        "OS_TARGET": "windows",
        "LOCALE": "pt-BR,en-US",
        "BLOCK_IMAGES": "true",
        "BLOCK_WEBRTC": "false",
        "DISABLE_COOP": "true",
        "PROXY_SERVER": "http://proxy:8080",
        "PROXY_USERNAME": "alice",
        "PROXY_PASSWORD": "secret",
    })
    assert kwargs["headless"] is True
    assert kwargs["geoip"] is True
    assert kwargs["os"] == "windows"
    assert kwargs["locale"] == ["pt-BR", "en-US"]
    assert kwargs["block_images"] is True
    assert kwargs["block_webrtc"] is False
    assert kwargs["disable_coop"] is True
    assert kwargs["proxy"] == {
        "server": "http://proxy:8080",
        "username": "alice",
        "password": "secret",
    }


def test_invalid_inputs_raise_value_error():
    with pytest.raises(ValueError):
        parse_bool_env("maybe")
    with pytest.raises(ValueError):
        parse_headless_env("desktop")
    with pytest.raises(ValueError):
        parse_viewer_resolution({"VIEWER_RESOLUTION": "1920"})
    with pytest.raises(ValueError):
        build_camoufox_kwargs({"OS_TARGET": "freebsd"})


def test_viewer_settings_and_virtual_display_override():
    assert parse_viewer_enabled({}) is True
    assert parse_viewer_enabled({"HEADLESS": "virtual"}) is True
    assert parse_viewer_enabled({"HEADLESS": "false"}) is True
    assert parse_viewer_enabled({"HEADLESS": "true"}) is False
    assert parse_viewer_enabled({"HEADLESS": "virtual", "ENABLE_VIEWER": "false"}) is False
    assert parse_viewer_enabled({"HEADLESS": "true", "ENABLE_VIEWER": "true"}) is False

    assert parse_viewer_resolution({}) == (1920, 1080)
    assert parse_viewer_resolution({"VIEWER_RESOLUTION": "1280x720"}) == (1280, 720)

    kwargs = build_camoufox_kwargs(
        {"HEADLESS": "virtual", "GEOIP": "true"},
        virtual_display_override=":99",
    )
    assert kwargs == {
        "host": "127.0.0.1",
        "port": 9224,
        "ws_path": "/camoufox",
        "headless": False,
        "virtual_display": ":99",
        "geoip": True,
    }
