FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    XDG_CACHE_HOME=/opt/camoufox-cache \
    BOOTSTRAP_STATUS_FILE=/tmp/camoufox-bootstrap.json \
    PORT=9223 \
    INTERNAL_WS_PORT=9224 \
    INTERNAL_WS_PATH=/camoufox \
    SELKIES_PORT=8080

WORKDIR /app

COPY requirements.txt /app/requirements.txt

RUN pip install --no-cache-dir --no-compile -r /app/requirements.txt

RUN printf 'path-exclude=/usr/share/doc/*\npath-exclude=/usr/share/man/*\npath-exclude=/usr/share/groff/*\npath-exclude=/usr/share/info/*\npath-exclude=/usr/share/lintian/*\npath-exclude=/usr/share/locale/*\npath-include=/usr/share/locale/en*\n' > /etc/dpkg/dpkg.cfg.d/01_nodoc \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        libasound2 \
        libatk1.0-0 \
        libcairo-gobject2 \
        libcairo2 \
        libdbus-1-3 \
        libdbus-glib-1-2 \
        libfontconfig1 \
        libfreetype6 \
        libgdk-pixbuf-2.0-0 \
        libglib2.0-0 \
        libgtk-3-0 \
        libharfbuzz0b \
        libpango-1.0-0 \
        libpangocairo-1.0-0 \
        libx11-6 \
        libx11-xcb1 \
        libxcb-shm0 \
        libxcb1 \
        libxcomposite1 \
        libxcursor1 \
        libxdamage1 \
        libxext6 \
        libxfixes3 \
        libxi6 \
        libxrandr2 \
        libxrender1 \
        libxt6 \
        libxtst6 \
        libnss3 \
        libnspr4 \
        libgbm1 \
        libegl1 \
        libgl1 \
        libgl1-mesa-dri \
        libpixman-1-0 \
        libxkbcommon0 \
        libice6 \
        libsm6 \
        xvfb \
        openbox \
        x11-xserver-utils \
        x11-utils \
        xauth \
        util-linux \
        fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/* /usr/share/doc/* /usr/share/man/* /usr/share/icons/* \
    && python -c "import pixelflux, pcmflux, selkies, cryptography; print('Selkies native deps OK')"

RUN --mount=type=secret,id=GITHUB_TOKEN,required=false \
    if [ -s /run/secrets/GITHUB_TOKEN ]; then export GITHUB_TOKEN="$(cat /run/secrets/GITHUB_TOKEN)"; fi \
    && groupadd --system camoufox \
    && useradd --system --gid camoufox --create-home --home-dir /home/camoufox camoufox \
    && mkdir -p /opt/camoufox-cache /tmp/.X11-unix \
    && chmod 1777 /tmp/.X11-unix \
    && python -c "from camoufox.fpgen_model import ensure_fpgen_model; ensure_fpgen_model()" \
    && FPDATA_DIR=$(python -c "import importlib.util, pathlib; print(pathlib.Path(importlib.util.find_spec('fpgen').origin).parent / 'data')") \
    && chown -R camoufox:camoufox /opt/camoufox-cache /app "$FPDATA_DIR"

COPY --chown=camoufox:camoufox config_builder.py proxy.mjs server.py /app/

EXPOSE 9223

HEALTHCHECK --interval=30s --timeout=5s --start-period=300s --retries=5 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9223/health', timeout=5)"

CMD ["python", "-u", "/app/server.py"]
