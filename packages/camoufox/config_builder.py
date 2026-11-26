"""Pure environment configuration parser for the Camoufox Playwright + Selkies server."""

from __future__ import annotations

import os
import re
from typing import Any, Dict, Mapping, Optional, Tuple, Union

_RES_PATTERN = re.compile(r"^([1-9][0-9]{2,3})x([1-9][0-9]{2,3})$")


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


def parse_viewer_enabled(env: Mapping[str, str] | None = None) -> bool:
    """Enable Selkies live browser streaming when running on a virtual/headed display unless disabled."""
    source = os.environ if env is None else env
    headless = parse_headless_env(source.get("HEADLESS"))
    explicit = parse_bool_env(source.get("ENABLE_VIEWER"))
    if explicit is not None:
        return explicit and headless is not True
    return headless is not True


def parse_viewer_resolution(env: Mapping[str, str] | None = None) -> Tuple[int, int]:
    """Parse initial X11 resolution WxH for Xvfb/Selkies (default 1920x1080)."""
    source = os.environ if env is None else env
    raw = (source.get("VIEWER_RESOLUTION") or "").strip().lower()
    if not raw:
        return (1920, 1080)
    match = _RES_PATTERN.match(raw)
    if not match:
        raise ValueError(f"Invalid VIEWER_RESOLUTION {raw!r}; expected WIDTHxHEIGHT (e.g. 1920x1080).")
    return int(match.group(1)), int(match.group(2))


def build_camoufox_kwargs(
    env: Mapping[str, str] | None = None,
    virtual_display_override: Optional[str] = None,
) -> Dict[str, Any]:
    """Build keyword arguments compatible with camoufox launch_options/launch_server."""
    source = os.environ if env is None else env

    internal_port = int((source.get("INTERNAL_WS_PORT") or "9224").strip())
    internal_ws_path = (source.get("INTERNAL_WS_PATH") or "/camoufox").strip()
    if not internal_ws_path.startswith("/"):
        internal_ws_path = f"/{internal_ws_path}"

    headless_mode = parse_headless_env(source.get("HEADLESS"))
    kwargs: Dict[str, Any] = {
        "host": "127.0.0.1",
        "port": internal_port,
        "ws_path": internal_ws_path,
        "headless": False if virtual_display_override else headless_mode,
    }
    if virtual_display_override:
        kwargs["virtual_display"] = virtual_display_override

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
