# Dockhand

**Dockhand** is a modern, lightweight Docker management Web UI built for container control, visual Docker Compose orchestration, GitOps stack deployments, and multi-environment administration.

## Key Features

- **Container & Compose Management**: Start, stop, inspect, edit Compose stacks alongside environment variables, and monitor live container stats.
- **Terminal, Logs & Files**: Interactive web shell (`exec`), real-time log streaming, and an in-container file browser.
- **Dedicated PostgreSQL 16 Backend**: Uses an isolated `postgres:16-alpine` database service with healthcheck gating and a per-install randomly generated database password.
- **Multi-Environment & Security**: Manage the local Docker socket or connect remote hosts via TCP or Hawser agents, with Grype/Trivy vulnerability scanning.

## Configuration Notes

- **Database Credentials**: Enter your desired `POSTGRES_USER` (and optional `POSTGRES_DB`) during installation. Runtipi automatically generates a unique random hexadecimal password (`POSTGRES_PASSWORD`) and wires `DATABASE_URL` securely.
- **Port Mapping**: Exposed on host port `3180` by default (customizable in Runtipi), mapped internally to container port `3000` to avoid collisions with other port `3000` services.
