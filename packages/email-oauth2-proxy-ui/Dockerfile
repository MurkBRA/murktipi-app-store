# Stage 1: Build static Go binary
FROM golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY internal ./internal
COPY web ./web

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/email-oauth2-proxy-ui .

# Stage 2: Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/email-oauth2-proxy-ui /usr/local/bin/email-oauth2-proxy-ui

ENV LISTEN_ADDR=":8090" \
    CONFIG_DIR="/config" \
    PROXY_HOST="email-oauth2-proxy" \
    ADMIN_USER="admin"

EXPOSE 8090

ENTRYPOINT ["/usr/local/bin/email-oauth2-proxy-ui"]
