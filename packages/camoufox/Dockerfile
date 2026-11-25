FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    XDG_CACHE_HOME=/opt/camoufox-cache \
    PORT=9223 \
    INTERNAL_WS_PORT=9224 \
    INTERNAL_WS_PATH=/camoufox

WORKDIR /app

COPY requirements.txt /app/requirements.txt

RUN pip install --no-cache-dir -r /app/requirements.txt \
    && python -m playwright install-deps firefox \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        xvfb \
        util-linux \
        libgl1 \
        libgl1-mesa-dri \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system camoufox \
    && useradd --system --gid camoufox --create-home --home-dir /home/camoufox camoufox \
    && mkdir -p /opt/camoufox-cache \
    && chown -R camoufox:camoufox /opt/camoufox-cache /app

USER camoufox

RUN python -m camoufox fetch

COPY --chown=camoufox:camoufox config_builder.py proxy.mjs server.py /app/

EXPOSE 9223

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9223/health', timeout=5)"

CMD ["python", "-u", "/app/server.py"]
