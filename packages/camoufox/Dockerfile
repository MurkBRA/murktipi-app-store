# syntax=docker/dockerfile:1
FROM ghcr.io/linuxserver/baseimage-selkies:debiantrixie

ENV TITLE=Camoufox \
    NO_DECOR=true \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    XDG_CACHE_HOME=/config/.cache \
    BOOTSTRAP_STATUS_FILE=/tmp/camoufox-bootstrap.json \
    PORT=9223 \
    INTERNAL_WS_PORT=9224 \
    INTERNAL_WS_PATH=/camoufox \
    UI_MUX_PORT=3003 \
    SELKIES_HTTP_PORT=3000 \
    SELKIES_HTTPS_PORT=3001

WORKDIR /app

COPY requirements.txt /app/requirements.txt
COPY icon.png /usr/share/selkies/www/icon.png

RUN echo "**** install camoufox runtime deps ****" \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        libasound2t64 \
        libcairo-gobject2 \
        libdbus-glib-1-2 \
        libfontconfig1 \
        libgdk-pixbuf-2.0-0 \
        libglib2.0-0 \
        libharfbuzz0b \
        libpango-1.0-0 \
        libpangocairo-1.0-0 \
        libx11-xcb1 \
        libxcb-shm0 \
        libxcomposite1 \
        libxdamage1 \
        libxi6 \
        libxrandr2 \
        libxrender1 \
        libxt6t64 \
        libnspr4 \
        libpciaccess0 \
        libegl1 \
        libgl1 \
    && echo "**** install python packages into /lsiopy ****" \
    && /lsiopy/bin/pip install --no-cache-dir --no-compile -r /app/requirements.txt \
    && /lsiopy/bin/python -c "import pixelflux, pcmflux, selkies, cryptography, camoufox; print('LSIO Selkies + Camoufox deps OK')" \
    && apt-get autoclean \
    && rm -rf /var/lib/apt/lists/* /var/tmp/* /tmp/*

RUN --mount=type=secret,id=GITHUB_TOKEN,required=false \
    if [ -s /run/secrets/GITHUB_TOKEN ]; then export GITHUB_TOKEN="$(cat /run/secrets/GITHUB_TOKEN)"; fi \
    && /lsiopy/bin/python -c "from camoufox.fpgen_model import ensure_fpgen_model; ensure_fpgen_model()" \
    && FPDATA_DIR=$(/lsiopy/bin/python -c "import importlib.util, pathlib; print(pathlib.Path(importlib.util.find_spec('fpgen').origin).parent / 'data')") \
    && chown -R abc:abc /app "$FPDATA_DIR"

COPY --chown=abc:abc config_builder.py proxy.mjs server.py /app/
COPY root/ /

RUN chmod +x /defaults/autostart /usr/bin/wrapped-camoufox /etc/s6-overlay/s6-rc.d/svc-camoufox/run \
    && sed -i 's/\r$//' /defaults/autostart /defaults/menu.xml /usr/bin/wrapped-camoufox \
        /etc/s6-overlay/s6-rc.d/svc-camoufox/run /etc/s6-overlay/s6-rc.d/svc-camoufox/type

EXPOSE 3000 3001 3003 9223

VOLUME /config

HEALTHCHECK --interval=30s --timeout=5s --start-period=300s --retries=5 \
    CMD /lsiopy/bin/python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9223/health', timeout=5)"
