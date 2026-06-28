---
name: data-proxy-local-deploy
description: Build data-proxy locally, package a linux/amd64 Docker image, upload it to the production server by SFTP or SCP, deploy it with the production compose files including WeChat Pay secrets, verify health, preserve rollback image archives, and avoid GitHub artifact downloads. Use when the user asks to deploy data-proxy from the local machine, upload a locally built image, or update dp.app.mbu.ltd without pulling from GitHub.
metadata:
  short-description: Local Docker build, upload, deploy, verify
---

# Data Proxy Local Deploy

Use this for production deploys where GitHub artifact downloads are slow or unavailable. Build on the local machine, upload the image archive, then switch the server container in place.

## Defaults

- Local repo: `/Users/fushilu/workspace/revocloud/data-proxy/upstream/new-api`
- Remote app dir: `/root/workspace/dataproxy/data-proxy`
- Remote image archive dir: `/root/workspace/dataproxy/image-archive`
- Service/container: `data-proxy`
- Public domain: `https://dp.app.mbu.ltd`
- Local app health: `http://127.0.0.1:13002/api/status`
- Public health: `https://dp.app.mbu.ltd/api/status`
- Compose files: `docker-compose.prod.yml` and `docker-compose.wechat-pay.yml`
- Electerm MCP endpoint, when available: `http://127.0.0.1:30837/mcp`
- Electerm bookmark usually used: `snsc-prod-应用2`

Never print, commit, package, or upload local secrets except to their intended server secret path. Do not commit `.env.production`, WeChat Pay certs, API keys, private keys, or generated image archives.

## Local Build

1. Work from the repo root and inspect state:

```bash
git status --short
git rev-parse --short HEAD
```

2. Verify Docker context safety before building:

```bash
git check-ignore -v .env.production
sed -n '1,220p' .dockerignore
```

`.env.production`, `.env.*`, `data/`, `logs/`, `output/`, `.agents/`, and local archives must be ignored.

3. Build and save a linux/amd64 image:

```bash
COMMIT="$(git rev-parse --short HEAD)"
TAG="local-${COMMIT}-$(date -u +%Y%m%d%H%M%S)"
IMAGE="data-proxy:${TAG}"
ARCHIVE="/tmp/data-proxy-${TAG}-linux-amd64.tar.gz"

docker buildx build --platform linux/amd64 --load -t "$IMAGE" .
docker save "$IMAGE" | gzip -1 > "$ARCHIVE"
shasum -a 256 "$ARCHIVE" > "${ARCHIVE%.tar.gz}.sha256"
```

Use `docker build --platform linux/amd64` only if buildx is unavailable.

## Upload

Upload the image archive, sha256 file, and any changed production deploy scripts to the remote app dir. Prefer full remote filenames.

Typical remote files:

```text
/root/workspace/dataproxy/data-proxy/data-proxy-<tag>-linux-amd64.tar.gz
/root/workspace/dataproxy/data-proxy/data-proxy-<tag>-linux-amd64.sha256
/root/workspace/dataproxy/data-proxy/scripts/prod-deploy.sh
/root/workspace/dataproxy/data-proxy/scripts/prod-compose.sh
/root/workspace/dataproxy/data-proxy/scripts/prod-ops-lib.sh
/root/workspace/dataproxy/data-proxy/scripts/prod-rollback.sh
/root/workspace/dataproxy/data-proxy/docker-compose.prod.yml
/root/workspace/dataproxy/data-proxy/docker-compose.wechat-pay.yml
```

If using Electerm MCP, list bookmarks/tabs first when the tab id is unknown, then use the SFTP upload tool. If SCP/SSH is already configured and reliable, it is fine to use `scp`/`ssh`.

## Remote Deploy

On the server:

```bash
cd /root/workspace/dataproxy/data-proxy
sha256sum -c data-proxy-<tag>-linux-amd64.sha256
chmod +x scripts/prod-*.sh
DATA_PROXY_IMAGE="data-proxy:<tag>" scripts/prod-deploy.sh ./data-proxy-<tag>-linux-amd64.tar.gz
```

`scripts/prod-deploy.sh` archives the currently running image before switching, loads the uploaded archive, runs:

```bash
docker compose -f docker-compose.prod.yml -f docker-compose.wechat-pay.yml up -d data-proxy
```

and waits for `http://127.0.0.1:13002/api/status`.

## Verify

After deployment:

```bash
docker ps --filter name=data-proxy --format '{{.Names}} {{.Image}} {{.Status}} {{.Ports}}'
curl -fsS http://127.0.0.1:13002/api/status
curl -kfsS https://dp.app.mbu.ltd/api/status
ls -lt /root/workspace/dataproxy/image-archive | head
```

Wait for Docker health to become `healthy`, not merely `starting`, unless the app health endpoint has already passed and the healthcheck is still within its first interval.

## Rollback

The deploy script writes rollback images to `/root/workspace/dataproxy/image-archive`. Roll back to the newest archive with:

```bash
cd /root/workspace/dataproxy/data-proxy
scripts/prod-rollback.sh
```

Or pass a specific archive:

```bash
scripts/prod-rollback.sh /root/workspace/dataproxy/image-archive/<archive>.tar
```

## Report

Report the deployed image tag, Git commit, upload method, local/public health result, and newest rollback archive path.
