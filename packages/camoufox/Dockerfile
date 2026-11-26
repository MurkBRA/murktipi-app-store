# syntax=docker/dockerfile:1
FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    HOME=/config \
    XDG_CACHE_HOME=/config/.cache \
    BOOTSTRAP_STATUS_FILE=/tmp/camoufox-bootstrap.json \
    PORT=9223 \
    INTERNAL_WS_PORT=9224 \
    INTERNAL_WS_PATH=/camoufox

WORKDIR /app

COPY requirements.txt /app/requirements.txt

RUN apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        libasound2t64 \
        libatk-bridge2.0-0t64 \
        libatk1.0-0t64 \
        libcairo-gobject2 \
        libcairo2 \
        libdbus-glib-1-2 \
        libegl1 \
        libfontconfig1 \
        libgbm1 \
        libgdk-pixbuf-2.0-0 \
        libgl1 \
        libglib2.0-0t64 \
        libgtk-3-0t64 \
        libharfbuzz0b \
        libnspr4 \
        libnss3 \
        libpango-1.0-0 \
        libpangocairo-1.0-0 \
        libpciaccess0 \
        libx11-xcb1 \
        libxcb-shm0 \
        libxcomposite1 \
        libxdamage1 \
        libxi6 \
        libxrandr2 \
        libxrender1 \
        libxshmfence1 \
        libxt6t64 \
        xvfb \
    && pip install --no-cache-dir --no-compile -r /app/requirements.txt \
    && python -c "import camoufox, playwright; print('Official Camoufox deps OK')" \
    && rm -rf /var/lib/apt/lists/* /var/tmp/* /tmp/*

RUN --mount=type=secret,id=GITHUB_TOKEN,required=false \
    if [ -s /run/secrets/GITHUB_TOKEN ]; then export GITHUB_TOKEN="$(cat /run/secrets/GITHUB_TOKEN)"; fi \
    && python -c "from camoufox.fpgen_model import ensure_fpgen_model; ensure_fpgen_model()" \
    && FPDATA_DIR=$(python -c "import importlib.util, pathlib; print(pathlib.Path(importlib.util.find_spec('fpgen').origin).parent / 'data')") \
    && chmod -R a+rX /app "$FPDATA_DIR" \
    && mkdir -p /config/.cache \
    && chmod 0777 /config /config/.cache

COPY --chmod=0644 config_builder.py proxy.mjs server.py /app/

EXPOSE 9223

VOLUME /config

HEALTHCHECK --interval=30s --timeout=5s --start-period=300s --retries=5 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9223/health', timeout=5)"

CMD ["python", "-u", "/app/server.py"]
