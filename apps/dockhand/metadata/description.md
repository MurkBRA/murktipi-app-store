# Dockhand

**Dockhand** is a modern, lightweight Docker management Web UI built for container control, visual Docker Compose orchestration, GitOps stack deployments, and multi-environment administration.

Because it runs as a single container directly on `tipi_main_network` using Dockhand's built-in SQLite storage (`/app/data`), Runtipi does not inject extra per-app IPAM subnets or sidecar database dependencies.

## Key Features

- **Container & Compose Management**: Start, stop, inspect, edit Compose stacks alongside environment variables, and monitor live container stats.
- **Terminal, Logs & Files**: Interactive web shell (`exec`), real-time log streaming, and an in-container file browser.
- **Built-in Persistent Storage**: Stores configuration, environments, and SQLite state directly inside `${APP_DATA_DIR}/data/dockhand:/app/data`.
- **Multi-Environment & Security**: Connect the local `/var/run/docker.sock` socket or remote hosts via TCP or Hawser agents, with Grype/Trivy vulnerability scanning.

## First-Time Setup (Required)

Dockhand does not auto-create a Docker environment on initial startup, so containers and stacks remain hidden until an environment is added:

1. Open Dockhand and go to **Settings > Environments > Add environment**.
2. Choose **Unix socket** with path `/var/run/docker.sock`.
3. Click **Test connection** and save. Host containers and active Docker Compose stacks will appear immediately.

## Configuration Notes

- **Port Mapping**: Exposed on host port `3180` by default (customizable in Runtipi), mapped internally to container port `3000` to avoid collisions with other port `3000` services.
