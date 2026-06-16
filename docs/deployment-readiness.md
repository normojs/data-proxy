# Deployment Readiness

Use the deployment preflight before tagging or handing a build to an operator:

```bash
make deployment-preflight
```

Use `docs/data-proxy-operator-guide.md` as the primary Data Proxy deployment
handoff. The top-level README files are preserved as upstream New API
attribution material and are not the recommended operator entry point.

The default gate runs:

- `go test ./...`
- `make build-all-frontends` (currently aliases the new frontend build)
- `docker compose config`
- `docker compose -f docker-compose.dev.yml config`
- `docker version`
- `docker buildx version`
- `git diff --check`

`make build-all-frontends` creates `web/default/dist`. The repository now keeps
only the newer `web/default` frontend; release and deployment builds no longer
build or package a legacy UI.

Before preparing operator-facing release notes or images, apply the branding
rules in `docs/branding-and-release-policy.md`: runtime surfaces use Data Proxy,
while required upstream attribution and preserved upstream documentation keep
their New API references.

## Optional Docker Image Build

The full Docker multi-stage image build is intentionally opt-in because it can
block on Docker Hub base image metadata or network availability before local
project code is compiled.

Run it when the release machine has reliable registry access:

```bash
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight
```

The default optional build command is equivalent to:

```bash
docker build --target builder2 -t data-proxy:preflight-builder .
```

Override the target or local tag when needed:

```bash
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 \
DEPLOYMENT_PREFLIGHT_DOCKER_TARGET=builder2 \
DEPLOYMENT_PREFLIGHT_IMAGE=data-proxy:release-preflight \
make deployment-preflight
```

## Current Status

The current release gate is green on this machine as of 2026-06-16.

Validated commands:

- `gtimeout 15 docker version`
- `gtimeout 15 docker info`
- `make deployment-preflight`
- `docker build --target builder2 -t data-proxy:preflight-builder .`
- `DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight`

Historical caveat: Docker Desktop previously reported `Docker Desktop is unable
to start`, and `docker version` / `docker info` timed out while reading Server
details. That local daemon issue later cleared without a code change. If it
returns on another release host, treat it as a host Docker recovery task before
tagging or publishing an image.

Note: the first full optional preflight retry after Docker recovered was killed
by the host while repeating the local frontend build (`SIGKILL`) before it
reached Docker build. The same frontend build had already passed in the default
preflight, the direct Docker build passed, and the full optional preflight passed
on retry with cached Docker layers.
