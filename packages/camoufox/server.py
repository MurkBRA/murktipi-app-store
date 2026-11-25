"""Supervisor entrypoint launching the Camoufox Playwright server and HTTP/WS proxy."""

from __future__ import annotations

import base64
import importlib.metadata
import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Optional

import orjson
from camoufox.server import LAUNCH_SCRIPT, get_nodejs, to_camel_case_dict
from camoufox.utils import launch_options, start_virtual_display
from camoufox.virtdisplay import VirtualDisplay

from config_builder import build_camoufox_kwargs

_STOPPING = False


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


def _launch_camoufox_once() -> tuple[subprocess.Popen, Optional[VirtualDisplay]]:
    kwargs = build_camoufox_kwargs()
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

    nodejs = get_nodejs()
    proxy_script = Path(__file__).resolve().with_name("proxy.mjs")
    proxy_env = os.environ.copy()
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

    camoufox_proc: Optional[subprocess.Popen] = None
    virtual_display: Optional[VirtualDisplay] = None

    try:
        while not _STOPPING:
            if proxy_proc.poll() is not None:
                print(f"[camoufox-supervisor] Proxy exited unexpectedly ({proxy_proc.returncode}), restarting...", flush=True)
                proxy_proc = subprocess.Popen(
                    [nodejs, str(proxy_script)],
                    env=proxy_env,
                    stdout=sys.stdout,
                    stderr=sys.stderr,
                )

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
                camoufox_proc, virtual_display = _launch_camoufox_once()

            time.sleep(0.5)
    finally:
        _stop_process(camoufox_proc, close_stdin=True)
        if virtual_display:
            virtual_display.kill()
        _stop_process(proxy_proc)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
