import re
from pathlib import Path

PKG_DIR = Path(__file__).resolve().parents[1]


def test_default_nginx_conf_routes():
    conf = (PKG_DIR / "default.conf").read_text(encoding="utf-8")
    assert "listen 80;" in conf
    assert "location = /" in conf
    assert "return 200 '<meta name=\"color-scheme\" content=\"dark\"><title>404</title>404';" in conf
    assert "location ~ ^/error\\.html/?$" in conf
    assert "return 301 /;" in conf
    assert "proxy_pass http://runtipi-reverse-proxy:80;" in conf


def test_dockerfile_and_entrypoint():
    df = (PKG_DIR / "Dockerfile").read_text(encoding="utf-8")
    assert re.search(r"ARG\s+NGROK_VERSION=3\.39\.13", df)
    assert "FROM nginx:1.31.10-alpine" in df
    assert "cp -L /bin/ngrok /tmp/ngrok-bin" in df
    assert "ENTRYPOINT [\"/app/entrypoint.sh\"]" in df

    ep = (PKG_DIR / "entrypoint.sh").read_text(encoding="utf-8")
    assert "ENABLE_NGINX_FILTER" in ep
    assert "TARGET=\"127.0.0.1:80\"" in ep
    assert "TARGET=\"${NGROK_TARGET:-80}\"" in ep
    assert "sed 's|^data:[^,]*,||'" in ep
    assert "--url=\"$NGROK_URL\"" in ep
