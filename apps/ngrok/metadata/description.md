# Ngrok Tunnel Agent

**Ngrok** exposes local services and Runtipi containers over secure HTTPS tunnels using the official **`ngrok/ngrok:alpine`** image, with the local web inspector available on `${APP_PORT}` (default **`4040`**).

Because it runs as a single container directly on `tipi_main_network`, Runtipi does not generate extra per-app IPAM subnets or `depends_on` dependencies.

## Configuration

The container executes:

```sh
ngrok http --url=${NGROK_URL} ${NGROK_TARGET:-80}
```

- **`NGROK_AUTHTOKEN` (Required):** Ngrok authentication token from your Ngrok dashboard.
- **`NGROK_URL` (Required):** Static Ngrok domain passed to `--url` (for example `my-url.ngrok-free.dev`).
- **`NGROK_TARGET` (Optional):** Upstream port or `service:port` target (defaults to `80` when left blank; supports values such as `80`, `8080`, `runtipi-reverse-proxy:80`, or `<container>:<port>`).
- **`CONTAINER_NAME` (Optional):** Custom Docker container name (defaults to `ngrok`).
