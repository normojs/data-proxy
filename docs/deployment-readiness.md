# Deployment Readiness

Use the deployment preflight before tagging or handing a build to an operator:

```bash
make deployment-preflight
```

The default gate runs:

- `go test ./...`
- `make build-all-frontends`
- `docker compose config`
- `docker compose -f docker-compose.dev.yml config`
- `docker version`
- `docker buildx version`
- `git diff --check`

`make build-all-frontends` creates `web/default/dist` and `web/classic/dist`.
Those directories are ignored by git and should stay uncommitted unless the
repository policy changes.

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
docker build --target builder2 -t new-api:preflight-builder .
```

Override the target or local tag when needed:

```bash
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 \
DEPLOYMENT_PREFLIGHT_DOCKER_TARGET=builder2 \
DEPLOYMENT_PREFLIGHT_IMAGE=new-api:release-preflight \
make deployment-preflight
```

## Current Caveat

The latest local preflight passed backend tests, both production frontend
builds, production/dev Compose config validation, Docker engine/buildx checks,
and whitespace checks. A full local Docker image build was attempted but had to
be canceled after stalling on Docker Hub metadata pulls for the pinned
`golang:1.26.1-alpine` and `oven/bun:1` base images.

Before publishing a release image, rerun the opt-in Docker image build from a
network environment that can pull those base images.
