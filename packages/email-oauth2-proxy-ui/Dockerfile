# Stage 1: Native cross-compile on host ($BUILDPLATFORM) without QEMU emulation
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod main.go ./
COPY internal ./internal
COPY web ./web

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/email-oauth2-proxy-ui .

# Stage 2: Zero-OS minimal runtime image (~8 MB total, 0 shell, 0 CVEs)
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/email-oauth2-proxy-ui /email-oauth2-proxy-ui

ENV LISTEN_ADDR=":8090" \
    CONFIG_DIR="/config" \
    COMPOSE_DIR="/compose" \
    PROXY_HOST="email-oauth2-proxy" \
    ADMIN_USER="admin"

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=10s \
    CMD ["/email-oauth2-proxy-ui", "-healthcheck"]

ENTRYPOINT ["/email-oauth2-proxy-ui"]
