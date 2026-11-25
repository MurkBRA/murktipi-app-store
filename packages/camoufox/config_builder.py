"""Pure environment configuration parser for the Camoufox Playwright server."""

from __future__ import annotations

import os
from typing import Any, Dict, Mapping, Optional, Union


def parse_bool_env(value: Optional[str]) -> Optional[bool]:
    """Parse an optional boolean environment value ('true'/'false')."""
    if value is None:
        return None
    normalized = value.strip().lower()
    if not normalized:
        return None
    if normalized in ("1", "true", "yes", "on"):
        return True
    if normalized in ("0", "false", "no", "off"):
        return False
    raise ValueError(f"Invalid boolean value: {value!r}")


def parse_headless_env(value: Optional[str]) -> Union[str, bool]:
    """Resolve HEADLESS env var ('virtual', 'true', or 'false')."""
    raw = (value or "virtual").strip().lower()
    if raw == "virtual":
        return "virtual"
    if raw in ("true", "1", "yes"):
        return True
    if raw in ("false", "0", "no"):
        return False
    raise ValueError(f"Unsupported HEADLESS mode {value!r}; expected 'virtual', 'true', or 'false'.")


def build_camoufox_kwargs(env: Mapping[str, str] | None = None) -> Dict[str, Any]:
    """Build keyword arguments compatible with camoufox launch_options/launch_server."""
    source = os.environ if env is None else env

    internal_port = int((source.get("INTERNAL_WS_PORT") or "9224").strip())
    internal_ws_path = (source.get("INTERNAL_WS_PATH") or "/camoufox").strip()
    if not internal_ws_path.startswith("/"):
        internal_ws_path = f"/{internal_ws_path}"

    kwargs: Dict[str, Any] = {
        "host": "127.0.0.1",
        "port": internal_port,
        "ws_path": internal_ws_path,
        "headless": parse_headless_env(source.get("HEADLESS")),
    }

    geoip = parse_bool_env(source.get("GEOIP"))
    if geoip is not None:
        kwargs["geoip"] = geoip

    os_target = (source.get("OS_TARGET") or "").strip().lower()
    if os_target:
        if os_target not in ("windows", "macos", "linux"):
            raise ValueError(f"Unsupported OS_TARGET {os_target!r}; expected 'windows', 'macos', or 'linux'.")
        kwargs["os"] = os_target

    locale_raw = (source.get("LOCALE") or "").strip()
    if locale_raw:
        locales = [item.strip() for item in locale_raw.split(",") if item.strip()]
        if len(locales) == 1:
            kwargs["locale"] = locales[0]
        elif len(locales) > 1:
            kwargs["locale"] = locales

    for env_key, kw_key in (
        ("BLOCK_IMAGES", "block_images"),
        ("BLOCK_WEBRTC", "block_webrtc"),
        ("DISABLE_COOP", "disable_coop"),
    ):
        parsed = parse_bool_env(source.get(env_key))
        if parsed is not None:
            kwargs[kw_key] = parsed

    proxy_server = (source.get("PROXY_SERVER") or "").strip()
    if proxy_server:
        proxy: Dict[str, str] = {"server": proxy_server}
        proxy_user = (source.get("PROXY_USERNAME") or "").strip()
        proxy_pass = (source.get("PROXY_PASSWORD") or "").strip()
        if proxy_user:
            proxy["username"] = proxy_user
        if proxy_pass:
            proxy["password"] = proxy_pass
        kwargs["proxy"] = proxy

    return kwargs
