# Data Proxy Operator Guide

This guide is the primary operator handoff for Data Proxy deployments. The
top-level README files are preserved as upstream New API attribution material;
use this document for Data Proxy runtime setup and release checks.

## Quick Start With Docker Compose

Start only the Data Proxy service first:

```bash
docker compose up -d data-proxy
```

Then open:

```text
http://localhost:3000
```

On a fresh install, the first-run setup wizard asks you to prepare the runtime
workspace before the first administrator account is created.

## Production Compose Entry Point

Production servers should use the provided scripts instead of calling
`docker compose -f docker-compose.prod.yml ...` directly. The wrapper always
loads both `docker-compose.prod.yml` and `docker-compose.wechat-pay.yml`, so
the WeChat Pay merchant key mount stays present after every restart, deploy,
and rollback:

```bash
scripts/prod-compose.sh ps
scripts/prod-compose.sh logs -f data-proxy
scripts/prod-compose.sh up -d data-proxy
```

The WeChat Pay override mounts:

```text
./secrets/wechatpay -> /run/secrets/data-proxy/wechatpay:ro
```

Deploy a new image with one of these forms:

```bash
scripts/prod-deploy.sh ./data-proxy-<tag>.tar
scripts/prod-deploy.sh ghcr.io/normojs/data-proxy:<tag>
```

Before switching images, `prod-deploy.sh` saves the currently running
`data-proxy` container image to:

```text
/root/workspace/dataproxy/image-archive
```

The default retention is the newest 10 archives. Override it only when the
server has enough disk space:

```bash
DATA_PROXY_IMAGE_ARCHIVE_KEEP=20 scripts/prod-deploy.sh ./data-proxy-<tag>.tar
```

If a deployment is unhealthy, roll back to the newest archived image:

```bash
scripts/prod-rollback.sh
```

You can also roll back to a specific archive or registry image:

```bash
scripts/prod-rollback.sh /root/workspace/dataproxy/image-archive/<archive>.tar
scripts/prod-rollback.sh ghcr.io/normojs/data-proxy:<previous-tag>
```

Both deploy and rollback run a default health check against
`http://127.0.0.1:13002/api/status`. Override with
`DATA_PROXY_HEALTH_URL` when the service binds a different local port. If the
server needs another compose override, append it with
`DATA_PROXY_EXTRA_COMPOSE_FILES`, separated by `:`.

## Database And Redis Choices

Data Proxy supports two first-run dependency modes.

Use existing services when you already run MySQL, PostgreSQL, or Redis:

- If Data Proxy runs in Docker and the dependency runs on this Mac, use
  `host.docker.internal` as the dependency host.
- If Data Proxy runs directly on the same machine as the dependency, use
  `127.0.0.1`.
- If the dependency runs on another host, use that network IP address or domain.

Use bundled local dependencies only when you want Compose-managed PostgreSQL
and Redis for this Data Proxy workspace:

```bash
docker compose --profile local-deps up -d
```

In the setup wizard, use these hosts:

- PostgreSQL host: `postgres`
- Redis host: `redis`

The bundled PostgreSQL and Redis services stay inside the Compose network and
do not publish host ports by default. They will not occupy `5432` or `6379` on
the Mac unless an operator explicitly adds port mappings.

## Runtime Config Restart

The first-run wizard writes database and Redis choices to the local runtime
config. When Data Proxy runs in Docker, the server schedules a controlled
process exit after saving that config so Docker Compose can restart the
container with the new database/Redis settings.

For non-container deployments, restart the process manually after saving
runtime config. Hot-reloading global database and Redis handles is intentionally
not used during first install.

Set this only when you need to disable automatic container restart behavior:

```bash
DATA_PROXY_SETUP_AUTO_RESTART=false
```

## Environment Variables

For first install, prefer the setup wizard over `SQL_DSN` and
`REDIS_CONN_STRING`.

Environment variables remain supported for advanced operations and explicit
overrides. Values from the environment win over saved runtime config. See
`.env.example` for examples, including Docker-to-host DSN formats.

## Local Validation

Before handing a build to an operator, run:

```bash
make deployment-preflight
```

The default gate runs backend tests, the new frontend build, Compose config
validation, Docker daemon checks, buildx checks, and whitespace checks.

When the release machine has reliable registry access, also run the optional
image build gate:

```bash
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight
```

See `docs/deployment-readiness.md` for the latest checked commands and current
machine status.

## Branding Boundary

Runtime surfaces, deployment docs, setup copy, and operator handoff material use
the Data Proxy brand.

Upstream New API references are still expected in source attribution, module
paths, preserved upstream README files, copyright notices, and compatibility
links. See `docs/branding-and-release-policy.md` before changing branding in
those areas.
