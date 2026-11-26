"""Supervisor entrypoint launching Xvfb, Openbox, Selkies viewer, Camoufox Playwright server, and proxy."""

from __future__ import annotations

import base64
import importlib.metadata
import os
import shutil
import signal
import socket
import subprocess
import sys
import time
from pathlib import Path
from typing import Optional

import orjson
from camoufox.server import LAUNCH_SCRIPT, get_nodejs, to_camel_case_dict
from camoufox.utils import launch_options, start_virtual_display
from camoufox.virtdisplay import VirtualDisplay

from config_builder import (
    build_camoufox_kwargs,
    parse_headless_env,
    parse_viewer_enabled,
    parse_viewer_resolution,
)

_STOPPING = False
_SHARED_DISPLAY = ":99"
_DISPLAY_SOCKET = "/tmp/.X11-unix/X99"


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


def _wait_for_unix_socket(sock_path: str, proc: subprocess.Popen, timeout_s: float = 20.0) -> None:
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            raise RuntimeError(f"Xvfb exited unexpectedly with code {proc.returncode}")
        if os.path.exists(sock_path):
            with socket.socket(socket.AF_UNIX) as sock:
                try:
                    sock.connect(sock_path)
                    return
                except OSError:
                    pass
        time.sleep(0.15)
    raise RuntimeError(f"Timed out waiting for X11 socket {sock_path}")


def _start_shared_xvfb() -> subprocess.Popen:
    xvfb_bin = shutil.which("Xvfb")
    if not xvfb_bin:
        raise RuntimeError("Xvfb binary not found in PATH")
    env = os.environ.copy()
    env["__GLX_VENDOR_LIBRARY_NAME"] = "mesa"
    env["LIBGL_ALWAYS_SOFTWARE"] = "1"
    proc = subprocess.Popen(
        [
            xvfb_bin,
            _SHARED_DISPLAY,
            "-screen",
            "0",
            "8192x4096x24",
            "-ac",
            "+extension",
            "GLX",
            "+extension",
            "RANDR",
            "-nolisten",
            "tcp",
            "-noreset",
            "-s",
            "0",
            "-dpms",
        ],
        env=env,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )
    _wait_for_unix_socket(_DISPLAY_SOCKET, proc)
    return proc


def _configure_display_resolution(display_env: dict[str, str], width: int, height: int) -> None:
    try:
        subprocess.run(
            [sys.executable, "-m", "selkies", "resize", f"{width}x{height}"],
            env=display_env,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=20,
        )
    except Exception as exc:
        print(f"[camoufox-supervisor] Warning: initial selkies resize failed: {exc}", flush=True)


def _start_openbox(display_env: dict[str, str]) -> Optional[subprocess.Popen]:
    openbox_bin = shutil.which("openbox")
    if not openbox_bin:
        print("[camoufox-supervisor] Warning: openbox not found; starting without window manager", flush=True)
        return None
    if shutil.which("xsetroot"):
        subprocess.run(["xsetroot", "-solid", "#0b1120"], env=display_env, check=False)
    return subprocess.Popen(
        [openbox_bin],
        env=display_env,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )


def _start_selkies(display_env: dict[str, str], selkies_port: int) -> subprocess.Popen:
    return subprocess.Popen(
        [
            sys.executable,
            "-m",
            "selkies",
            "--addr",
            "127.0.0.1",
            "--port",
            str(selkies_port),
            "--subfolder",
            "/viewer",
            "--enable-basic-auth=false",
            "--enable-https=false",
            "--audio-enabled=false",
            "--gamepad-enabled=false",
            "--printing-enabled=false",
            "--ui-title",
            "Camoufox Live Viewer",
        ],
        env=display_env,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )


def _launch_camoufox_once(
    shared_display: Optional[str] = None,
) -> tuple[subprocess.Popen, Optional[VirtualDisplay]]:
    kwargs = build_camoufox_kwargs(virtual_display_override=shared_display)
    virtual_display: Optional[VirtualDisplay] = None
    if not shared_display:
        virtual_display, kwargs["headless"] = start_virtual_display(
            kwargs.get("headless"), False, kwargs
        )
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
    except BaseException:
        if virtual_display:
            virtual_display.kill()
        raise


def main() -> int:
    global _STOPPING

    def handle_signal(signum, _frame):
        global _STOPPING
        print(f"[camoufox-supervisor] Received signal {signum}, shutting down...", flush=True)
        _STOPPING = True

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    viewer_enabled = parse_viewer_enabled()
    headless_mode = parse_headless_env(os.environ.get("HEADLESS"))
    selkies_port = int((os.environ.get("SELKIES_PORT") or "8080").strip())

    nodejs = get_nodejs()
    proxy_script = Path(__file__).resolve().with_name("proxy.mjs")
    proxy_env = os.environ.copy()
    proxy_env["VIEWER_ACTIVE"] = "true" if viewer_enabled else "false"
    proxy_env["SELKIES_PORT"] = str(selkies_port)
    try:
        proxy_env.setdefault("CAMOUFOX_LIB_VERSION", importlib.metadata.version("camoufox"))
    except importlib.metadata.PackageNotFoundError:
        pass

    proxy_proc = subprocess.Popen(
        [nodejs, str(proxy_script)],
        env=proxy_env,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )

    xvfb_proc: Optional[subprocess.Popen] = None
    openbox_proc: Optional[subprocess.Popen] = None
    selkies_proc: Optional[subprocess.Popen] = None
    camoufox_proc: Optional[subprocess.Popen] = None
    virtual_display: Optional[VirtualDisplay] = None
    display_env = os.environ.copy()
    shared_display: Optional[str] = None

    try:
        if viewer_enabled and headless_mode is not True:
            width, height = parse_viewer_resolution()
            print(f"[camoufox-supervisor] Starting Xvfb {_SHARED_DISPLAY} ({width}x{height}) + Selkies...", flush=True)
            xvfb_proc = _start_shared_xvfb()
            shared_display = _SHARED_DISPLAY
            display_env = os.environ.copy()
            display_env["DISPLAY"] = _SHARED_DISPLAY
            display_env["GDK_BACKEND"] = "x11"
            display_env["MOZ_ENABLE_WAYLAND"] = "0"
            display_env["__GLX_VENDOR_LIBRARY_NAME"] = "mesa"
            display_env["LIBGL_ALWAYS_SOFTWARE"] = "1"
            _configure_display_resolution(display_env, width, height)
            openbox_proc = _start_openbox(display_env)
            selkies_proc = _start_selkies(display_env, selkies_port)

        while not _STOPPING:
            if proxy_proc.poll() is not None:
                print(
                    f"[camoufox-supervisor] Proxy exited unexpectedly ({proxy_proc.returncode}), restarting...",
                    flush=True,
                )
                proxy_proc = subprocess.Popen(
                    [nodejs, str(proxy_script)],
                    env=proxy_env,
                    stdout=sys.stdout,
                    stderr=sys.stderr,
                )

            if shared_display and xvfb_proc is not None and xvfb_proc.poll() is not None:
                print("[camoufox-supervisor] Xvfb exited unexpectedly, restarting display stack...", flush=True)
                _stop_process(selkies_proc)
                _stop_process(openbox_proc)
                xvfb_proc = _start_shared_xvfb()
                width, height = parse_viewer_resolution()
                _configure_display_resolution(display_env, width, height)
                openbox_proc = _start_openbox(display_env)
                selkies_proc = _start_selkies(display_env, selkies_port)

            if shared_display and selkies_proc is not None and selkies_proc.poll() is not None:
                print(
                    f"[camoufox-supervisor] Selkies exited ({selkies_proc.returncode}), restarting in 2s...",
                    flush=True,
                )
                time.sleep(2)
                if not _STOPPING:
                    selkies_proc = _start_selkies(display_env, selkies_port)

            if camoufox_proc is None or camoufox_proc.poll() is not None:
                if camoufox_proc is not None:
                    print(
                        f"[camoufox-supervisor] Camoufox server exited ({camoufox_proc.returncode}), restarting in 2s...",
                        flush=True,
                    )
                    if virtual_display:
                        virtual_display.kill()
                        virtual_display = None
                    time.sleep(2)
                    if _STOPPING:
                        break
                print("[camoufox-supervisor] Launching Camoufox Playwright server...", flush=True)
                camoufox_proc, virtual_display = _launch_camoufox_once(shared_display=shared_display)

            time.sleep(0.5)
    finally:
        _stop_process(camoufox_proc, close_stdin=True)
        if virtual_display:
            virtual_display.kill()
        _stop_process(selkies_proc)
        _stop_process(openbox_proc)
        _stop_process(xvfb_proc)
        _stop_process(proxy_proc)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
