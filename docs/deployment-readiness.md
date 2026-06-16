# Deployment Readiness

Use the deployment preflight before tagging or handing a build to an operator:

```bash
make deployment-preflight
```

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

The latest code-level local checks passed backend tests, the production new
frontend build, production/dev Compose config validation, buildx availability,
and whitespace checks. The release image gate is still blocked by the local
Docker Desktop daemon state: `docker version` prints client information and
then hangs before server information, while Docker's Unix socket responds with
`Docker Desktop is unable to start`.

Recheck on 2026-06-16: `gtimeout 15 docker version` and `gtimeout 15 docker
info` still time out while reading Server information, and the Unix socket still
returns `Docker Desktop is unable to start`.

`docker desktop start` reports that Docker Desktop is already running, but the
daemon remains unhealthy: `docker version` / `docker info` still time out before
Server details and the Unix socket still returns `Docker Desktop is unable to
start`.

Follow-up non-Docker regression on 2026-06-16 passed: `go test ./...`,
`cd web/default && bun run typecheck`, locale JSON parsing,
`make build-all-frontends`, `make mcp-regression`, and a scoped whitespace check
that excluded current-session Fusion benchmark dirty files. This keeps the
code-level release signal green while Docker remains the only release-image
blocker.

Before publishing a release image, recover Docker Desktop on the release host,
then rerun the default preflight and the opt-in Docker image build. A release
tag should only be cut after both commands complete without hanging.
