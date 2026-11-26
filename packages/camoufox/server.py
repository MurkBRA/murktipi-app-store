"""Supervisor running the official Camoufox Playwright server and WS/HTTP proxy."""

from __future__ import annotations

import base64
import importlib.metadata
import json
import os
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Optional

import orjson
from camoufox.exceptions import CamoufoxNotInstalled
from camoufox.fpgen_model import ensure_fpgen_model
from camoufox.geolocation import download_mmdb, get_mmdb_path, needs_update
from camoufox.pkgman import CamoufoxFetcher, camoufox_path
from camoufox.server import LAUNCH_SCRIPT, get_nodejs, to_camel_case_dict
from camoufox.utils import launch_options, start_virtual_display

from config_builder import (
    build_camoufox_kwargs,
    parse_bool_env,
    parse_slim_fonts,
    resolve_uid_gid,
)

_STOPPING = False
_BOOTSTRAP_STATUS_FILE = Path(os.environ.get("BOOTSTRAP_STATUS_FILE") or "/tmp/camoufox-bootstrap.json")
_SLIM_FONT_MAX_BYTES = 5 * 1024 * 1024
_FONT_SUFFIXES = {".ttf", ".ttc", ".otf", ".woff", ".woff2"}


def _chown_tree(root: Path, uid: int, gid: int) -> None:
    root.mkdir(parents=True, exist_ok=True)
    os.chown(root, uid, gid)
    for dirpath, dirnames, filenames in os.walk(root):
        base = Path(dirpath)
        for name in dirnames:
            os.chown(base / name, uid, gid, follow_symlinks=False)
        for name in filenames:
            os.chown(base / name, uid, gid, follow_symlinks=False)


def _maybe_drop_privileges() -> None:
    """Drop root privileges via setpriv when PUID/PGID is set so /config matches host ownership."""
    if not hasattr(os, "geteuid") or os.geteuid() != 0:
        return
    if os.environ.get("_CAMOUFOX_PRIV_DROPPED") == "1":
        return

    ids = resolve_uid_gid()
    if ids is None:
        return

    uid, gid = ids
    _chown_tree(Path("/config"), uid, gid)
    setpriv_bin = shutil.which("setpriv") or "/usr/bin/setpriv"
    os.environ["_CAMOUFOX_PRIV_DROPPED"] = "1"
    os.environ.setdefault("HOME", "/config")
    os.environ.setdefault("XDG_CACHE_HOME", "/config/.cache")
    os.execv(
        setpriv_bin,
        [
            setpriv_bin,
            f"--reuid={uid}",
            f"--regid={gid}",
            "--clear-groups",
            sys.executable,
            "-u",
            __file__,
            *sys.argv[1:],
        ],
    )


def _stop_process(proc: Optional[subprocess.Popen], close_stdin: bool = False) -> None:
    if proc is None:
        return
    if close_stdin and proc.stdin and not proc.stdin.closed:
        try:
            proc.stdin.close()
        except OSError:
            pass
    if proc.poll() is not None:
        return
    try:
        proc.wait(timeout=10)
        return
    except subprocess.TimeoutExpired:
        pass
    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait()


def _stop_virtual_display(virtual_display: Any) -> None:
    if virtual_display is None:
        return
    try:
        virtual_display.kill()
    except Exception:
        pass


def _write_bootstrap_status(phase: str, message: str) -> None:
    try:
        _BOOTSTRAP_STATUS_FILE.parent.mkdir(parents=True, exist_ok=True)
        tmp = _BOOTSTRAP_STATUS_FILE.with_suffix(".tmp")
        tmp.write_text(json.dumps({"phase": phase, "message": message}), encoding="utf-8")
        tmp.replace(_BOOTSTRAP_STATUS_FILE)
    except OSError:
        pass


def _prune_large_fonts(browser_dir: Path, max_bytes: int = _SLIM_FONT_MAX_BYTES) -> None:
    fonts_dir = browser_dir / "fonts"
    if not fonts_dir.is_dir():
        return
    removed = 0
    freed_bytes = 0
    for font_file in fonts_dir.rglob("*"):
        if not font_file.is_file() or font_file.suffix.lower() not in _FONT_SUFFIXES:
            continue
        try:
            sz = font_file.stat().st_size
            if sz > max_bytes:
                font_file.unlink()
                removed += 1
                freed_bytes += sz
        except OSError:
            pass
    if removed:
        print(
            f"[camoufox-supervisor] SLIM_FONTS pruned {removed} font file(s) > {max_bytes // (1024 * 1024)} MiB "
            f"(freed {freed_bytes / (1024 * 1024):.1f} MiB in {fonts_dir})",
            flush=True,
        )


def _ensure_camoufox_runtime() -> bool:
    """Ensure the pinned Camoufox browser (and optional GeoIP DB) exists in /config/.cache."""
    try:
        browser_dir = Path(camoufox_path(download_if_missing=False))
        print(f"[camoufox-supervisor] Using cached Camoufox browser at {browser_dir}", flush=True)
    except CamoufoxNotInstalled:
        delays = (5, 15, 30, 60)
        browser_dir = None
        for attempt in range(len(delays) + 1):
            if _STOPPING:
                return False
            _write_bootstrap_status(
                "downloading",
                "First-run setup: downloading Camoufox browser (~1.2 GB) into persistent cache (/config/.cache)...",
            )
            print(
                f"[camoufox-supervisor] Camoufox browser not cached; downloading pinned build (attempt {attempt + 1})...",
                flush=True,
            )
            try:
                CamoufoxFetcher().install()
                browser_dir = Path(camoufox_path(download_if_missing=False))
                break
            except Exception as exc:
                if _STOPPING or attempt >= len(delays):
                    _write_bootstrap_status("error", f"Failed to download Camoufox browser: {exc}")
                    raise
                wait_s = delays[attempt]
                print(
                    f"[camoufox-supervisor] Download failed ({exc}); retrying in {wait_s}s...",
                    flush=True,
                )
                time.sleep(wait_s)
        assert browser_dir is not None

    if parse_slim_fonts():
        _prune_large_fonts(browser_dir)

    ensure_fpgen_model()

    if parse_bool_env(os.environ.get("GEOIP")):
        mmdb_path = get_mmdb_path()
        if not mmdb_path.exists() or needs_update():
            _write_bootstrap_status("downloading", "Downloading GeoIP database into /config/.cache...")
            print("[camoufox-supervisor] Fetching GeoIP database on demand...", flush=True)
            download_mmdb()

    return True


def _launch_camoufox_once() -> tuple[subprocess.Popen, Any]:
    kwargs = build_camoufox_kwargs()
    virtual_display, resolved_headless = start_virtual_display(kwargs.get("headless"), False, kwargs)
    kwargs["headless"] = resolved_headless
    try:
        config = launch_options(**kwargs)
        nodejs = get_nodejs()
        driver_package = Path(nodejs).parent / "package"
        payload = base64.b64encode(orjson.dumps(to_camel_case_dict(config))).decode() + "\n"

        proc = subprocess.Popen(
            [nodejs, str(LAUNCH_SCRIPT), str(driver_package)],
            cwd=driver_package,
            stdin=subprocess.PIPE,
            stdout=sys.stdout,
            stderr=sys.stderr,
            text=True,
        )
        assert proc.stdin is not None
        proc.stdin.write(payload)
        proc.stdin.flush()
        return proc, virtual_display
    except Exception:
        _stop_virtual_display(virtual_display)
        raise


def main() -> int:
    global _STOPPING

    _maybe_drop_privileges()

    def handle_signal(signum, _frame):
        global _STOPPING
        print(f"[camoufox-supervisor] Received signal {signum}, shutting down...", flush=True)
        _STOPPING = True

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    os.environ.setdefault("HOME", "/config")
    os.environ.setdefault("XDG_CACHE_HOME", "/config/.cache")
    Path(os.environ["XDG_CACHE_HOME"]).mkdir(parents=True, exist_ok=True)
    os.environ["GDK_BACKEND"] = "x11"
    os.environ["MOZ_ENABLE_WAYLAND"] = "0"

    _write_bootstrap_status("starting", "Initializing Camoufox proxy...")

    node_bin = shutil.which("node") or get_nodejs()
    proxy_script = Path(__file__).resolve().with_name("proxy.mjs")
    proxy_env = os.environ.copy()
    proxy_env["BOOTSTRAP_STATUS_FILE"] = str(_BOOTSTRAP_STATUS_FILE)
    try:
        proxy_env.setdefault("CAMOUFOX_LIB_VERSION", importlib.metadata.version("camoufox"))
    except importlib.metadata.PackageNotFoundError:
        pass

    proxy_proc = subprocess.Popen(
        [node_bin, str(proxy_script)],
        env=proxy_env,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )

    camoufox_proc: Optional[subprocess.Popen] = None
    virtual_display: Any = None
    try:
        if not _ensure_camoufox_runtime() or _STOPPING:
            return 0
        _write_bootstrap_status("starting", "Starting Camoufox Playwright server...")

        while not _STOPPING:
            if proxy_proc.poll() is not None:
                print(
                    f"[camoufox-supervisor] Proxy exited unexpectedly ({proxy_proc.returncode}), restarting...",
                    flush=True,
                )
                proxy_proc = subprocess.Popen(
                    [node_bin, str(proxy_script)],
                    env=proxy_env,
                    stdout=sys.stdout,
                    stderr=sys.stderr,
                )

            if camoufox_proc is None or camoufox_proc.poll() is not None:
                if camoufox_proc is not None:
                    _stop_virtual_display(virtual_display)
                    virtual_display = None
                    print(
                        f"[camoufox-supervisor] Camoufox server exited ({camoufox_proc.returncode}), restarting in 2s...",
                        flush=True,
                    )
                    time.sleep(2)
                    if _STOPPING:
                        break
                print("[camoufox-supervisor] Launching Camoufox Playwright server...", flush=True)
                camoufox_proc, virtual_display = _launch_camoufox_once()
                _write_bootstrap_status("ready", "Ready")

            time.sleep(0.5)
    finally:
        _stop_process(camoufox_proc, close_stdin=True)
        _stop_virtual_display(virtual_display)
        _stop_process(proxy_proc)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
