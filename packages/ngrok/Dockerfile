# syntax=docker/dockerfile:1
ARG NGROK_VERSION=3.39.13

FROM ngrok/ngrok:${NGROK_VERSION}-alpine AS ngrok-src
USER root
RUN cp -L /bin/ngrok /tmp/ngrok-bin && chmod 0755 /tmp/ngrok-bin

FROM nginx:1.31.10-alpine
ARG NGROK_VERSION=3.39.13
LABEL org.opencontainers.image.title="Ngrok with Optional Nginx Filter" \
      org.opencontainers.image.version="${NGROK_VERSION}"

WORKDIR /app

COPY --from=ngrok-src /tmp/ngrok-bin /usr/local/bin/ngrok
COPY default.conf /app/default.conf
COPY entrypoint.sh /app/entrypoint.sh

RUN sed -i 's/\r$//' /app/default.conf /app/entrypoint.sh \
    && chmod +x /app/entrypoint.sh \
    && ngrok version \
    && nginx -v

EXPOSE 80 4040

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD nc -z 127.0.0.1 4040

ENTRYPOINT ["/app/entrypoint.sh"]
