FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    XDG_CACHE_HOME=/opt/camoufox-cache \
    PORT=9223 \
    INTERNAL_WS_PORT=9224 \
    INTERNAL_WS_PATH=/camoufox \
    SELKIES_PORT=8080

WORKDIR /app

COPY requirements.txt /app/requirements.txt

RUN pip install --no-cache-dir -r /app/requirements.txt

RUN python -m playwright install-deps firefox \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        xvfb \
        openbox \
        x11-xserver-utils \
        x11-utils \
        xauth \
        util-linux \
        libgl1 \
        libgl1-mesa-dri \
        libgbm1 \
        libegl1 \
        libpixman-1-0 \
        libxkbcommon0 \
        libxtst6 \
        libxi6 \
        libice6 \
        libsm6 \
        libx11-xcb1 \
    && rm -rf /var/lib/apt/lists/* \
    && python -c "import pixelflux, pcmflux, selkies, cryptography; print('Selkies native deps OK')"

RUN --mount=type=secret,id=GITHUB_TOKEN,required=false \
    if [ -s /run/secrets/GITHUB_TOKEN ]; then export GITHUB_TOKEN="$(cat /run/secrets/GITHUB_TOKEN)"; fi \
    && groupadd --system camoufox \
    && useradd --system --gid camoufox --create-home --home-dir /home/camoufox camoufox \
    && mkdir -p /opt/camoufox-cache \
    && python -m camoufox fetch \
    && FPDATA_DIR=$(python -c "import importlib.util, pathlib; print(pathlib.Path(importlib.util.find_spec('fpgen').origin).parent / 'data')") \
    && chown -R camoufox:camoufox /opt/camoufox-cache /app "$FPDATA_DIR"

USER camoufox

COPY --chown=camoufox:camoufox config_builder.py proxy.mjs server.py /app/

EXPOSE 9223

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9223/health', timeout=5)"

CMD ["python", "-u", "/app/server.py"]
