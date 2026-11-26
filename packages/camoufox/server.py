"""Supervisor running Camoufox Playwright server + proxy inside LSIO baseimage-selkies."""

from __future__ import annotations

import base64
import datetime
import importlib.metadata
import ipaddress
import json
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
from camoufox.exceptions import CamoufoxNotInstalled
from camoufox.fpgen_model import ensure_fpgen_model
from camoufox.geolocation import download_mmdb, get_mmdb_path, needs_update
from camoufox.pkgman import CamoufoxFetcher, camoufox_path
from camoufox.server import LAUNCH_SCRIPT, get_nodejs, to_camel_case_dict
from camoufox.utils import launch_options
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import ExtendedKeyUsageOID, NameOID

from config_builder import (
    build_camoufox_kwargs,
    parse_bool_env,
    parse_headless_env,
    parse_slim_fonts,
    parse_viewer_enabled,
)

_STOPPING = False
_DEFAULT_TLS_DIR = Path("/tmp/camoufox-tls")
_BOOTSTRAP_STATUS_FILE = Path(os.environ.get("BOOTSTRAP_STATUS_FILE") or "/tmp/camoufox-bootstrap.json")
_SLIM_FONT_MAX_BYTES = 5 * 1024 * 1024
_FONT_SUFFIXES = {".ttf", ".ttc", ".otf", ".woff", ".woff2"}


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


def _wait_for_x11_display(display: str, timeout_s: float = 30.0) -> None:
    display_num = display.lstrip(":").split(".")[0] or "1"
    sock_path = f"/tmp/.X11-unix/X{display_num}"
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline and not _STOPPING:
        if os.path.exists(sock_path):
            with socket.socket(socket.AF_UNIX) as sock:
                try:
                    sock.connect(sock_path)
                    return
                except OSError:
                    pass
        time.sleep(0.2)
    if not _STOPPING:
        print(f"[camoufox-supervisor] Warning: X11 socket {sock_path} not ready after {timeout_s}s", flush=True)


def _ensure_tls_cert() -> tuple[str, str]:
    """Use LSIO Nginx SSL cert (/config/ssl/cert.pem) when available, else generate a self-signed SAN cert."""
    for cert_cand, key_cand in (
        ((os.environ.get("TLS_CERT_PATH") or "").strip(), (os.environ.get("TLS_KEY_PATH") or "").strip()),
        ("/config/ssl/cert.pem", "/config/ssl/cert.key"),
    ):
        if cert_cand and key_cand and os.path.isfile(cert_cand) and os.path.isfile(key_cand):
            return cert_cand, key_cand

    cert_path = _DEFAULT_TLS_DIR / "cert.pem"
    key_path = _DEFAULT_TLS_DIR / "key.pem"
    if cert_path.is_file() and key_path.is_file():
        return str(cert_path), str(key_path)

    _DEFAULT_TLS_DIR.mkdir(parents=True, exist_ok=True)
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)

    san_entries: list[x509.GeneralName] = [
        x509.DNSName("localhost"),
        x509.IPAddress(ipaddress.ip_address("127.0.0.1")),
        x509.IPAddress(ipaddress.ip_address("::1")),
    ]
    seen_dns = {"localhost"}
    seen_ips = {"127.0.0.1", "::1"}

    hostname = socket.gethostname().strip()
    if hostname and hostname not in seen_dns:
        seen_dns.add(hostname)
        san_entries.append(x509.DNSName(hostname))

    extra_hosts = [h.strip() for h in (os.environ.get("TLS_HOSTS") or "").split(",") if h.strip()]
    for host in extra_hosts:
        try:
            ip_obj = ipaddress.ip_address(host)
            if str(ip_obj) not in seen_ips:
                seen_ips.add(str(ip_obj))
                san_entries.append(x509.IPAddress(ip_obj))
        except ValueError:
            if host not in seen_dns:
                seen_dns.add(host)
                san_entries.append(x509.DNSName(host))

    now = datetime.datetime.now(datetime.timezone.utc)
    subject = issuer = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "camoufox")])
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - datetime.timedelta(days=1))
        .not_valid_after(now + datetime.timedelta(days=3650))
        .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
        .add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]), critical=False)
        .add_extension(x509.SubjectAlternativeName(san_entries), critical=False)
        .sign(key, hashes.SHA256())
    )

    key_path.write_bytes(
        key.private_bytes(
            serialization.Encoding.PEM,
            serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        )
    )
    os.chmod(key_path, 0o600)
    cert_path.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    return str(cert_path), str(key_path)


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


def _launch_camoufox_once(shared_display: Optional[str]) -> subprocess.Popen:
    kwargs = build_camoufox_kwargs(virtual_display_override=shared_display)
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
    return proc


def main() -> int:
    global _STOPPING

    def handle_signal(signum, _frame):
        global _STOPPING
        print(f"[camoufox-supervisor] Received signal {signum}, shutting down...", flush=True)
        _STOPPING = True

    signal.signal(signal.SIGTERM, handle_signal)
    signal.signal(signal.SIGINT, handle_signal)

    os.environ.setdefault("XDG_CACHE_HOME", "/config/.cache")
    os.environ["GDK_BACKEND"] = "x11"
    os.environ["MOZ_ENABLE_WAYLAND"] = "0"

    viewer_enabled = parse_viewer_enabled()
    headless_mode = parse_headless_env(os.environ.get("HEADLESS"))
    display = os.environ.get("DISPLAY") or ":1"
    shared_display = display if (viewer_enabled and headless_mode is not True) else None

    _write_bootstrap_status("starting", "Initializing Camoufox proxy...")

    node_bin = shutil.which("node") or get_nodejs()
    proxy_script = Path(__file__).resolve().with_name("proxy.mjs")
    proxy_env = os.environ.copy()
    proxy_env["VIEWER_ACTIVE"] = "true" if viewer_enabled else "false"
    proxy_env["BOOTSTRAP_STATUS_FILE"] = str(_BOOTSTRAP_STATUS_FILE)
    try:
        cert_path, key_path = _ensure_tls_cert()
        proxy_env["TLS_CERT_PATH"] = cert_path
        proxy_env["TLS_KEY_PATH"] = key_path
    except Exception as exc:
        print(f"[camoufox-supervisor] Warning: TLS cert lookup/generation failed ({exc}); HTTP only on 9223", flush=True)
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
    try:
        if not _ensure_camoufox_runtime() or _STOPPING:
            return 0
        if shared_display:
            _wait_for_x11_display(shared_display)
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
                    print(
                        f"[camoufox-supervisor] Camoufox server exited ({camoufox_proc.returncode}), restarting in 2s...",
                        flush=True,
                    )
                    time.sleep(2)
                    if _STOPPING:
                        break
                print("[camoufox-supervisor] Launching Camoufox Playwright server...", flush=True)
                camoufox_proc = _launch_camoufox_once(shared_display=shared_display)
                _write_bootstrap_status("ready", "Ready")

            time.sleep(0.5)
    finally:
        _stop_process(camoufox_proc, close_stdin=True)
        _stop_process(proxy_proc)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
