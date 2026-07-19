# 生产部署证据 2026-07-20：`sha-e1e59279`

## 结论

**PASS**：`https://dp.app.mbu.ltd` 已从 `sha-5f695ffe` 升级到 **`sha-e1e59279`**（提交 `e1e59279`，含 lite Alpine/双 compose 等 main 最新内容）。

## 环境

| 项 | 值 |
| --- | --- |
| 主机 | `47.122.29.88`（snsc-prod-应用2） |
| 目录 | `/root/workspace/dataproxy/data-proxy` |
| 包 | `data-proxy-e1e59279-linux-amd64.tar.gz`（Package CI run `29668608093`） |
| SHA256 | `c13b3fee2ab4a07304153a6a31d046e342681d0097e12e24aee53dcf14a00192` |
| 回滚归档 | `image-archive/20260719T164036Z_data-proxy_5f695ffe.tar` |

## 步骤

1. `gh run download` 获取 Package 产物  
2. SFTP 上传 tar.gz / sha256 / `data-proxy-remote-deploy-e1e59279.sh`  
3. `sha256sum -c` OK；`prod-deploy.sh` load 镜像 `data-proxy:e1e59279`  
4. **注意**：服务器 `docker-compose.prod.yml` 曾**写死** `image: data-proxy:5f695ffe`，导致 `DATA_PROXY_IMAGE` 环境变量不生效  
5. 备份为 `docker-compose.prod.yml.bak.5f695ffe`，将 image 改为 `data-proxy:e1e59279`  
6. `prod-compose.sh up -d --force-recreate --no-deps data-proxy`  
7. 容器 **healthy**

## 验收

| 检查 | 结果 |
| --- | --- |
| 容器镜像 | `data-proxy:e1e59279` healthy |
| 本机 `/api/status` version | `sha-e1e59279` |
| 公网 `x-new-api-version` | `sha-e1e59279` |
| 公网 `/api/status` | `success=true`，`version=sha-e1e59279` |
| `/docs/one-click-deploy.md` | 200 / 4660B，UTF-8 text |
| `/docs/deploy-profiles.md` | 200 / 6447B |
| `/docs/user-quickstart.md` | 200 / 3988B |
| `/agent/install.sh` | 200 shell（非 SPA） |
| `/.well-known/openid-configuration` | 200 JSON |

## 运维备注

- 生产 compose 若继续硬编码 `image: data-proxy:<sha>`，每次部署需改文件或恢复为 `${DATA_PROXY_IMAGE:-…}` 以便 `prod-deploy.sh` 生效。  
- 旧镜像 `5f695ffe` 已归档，可用 `scripts/prod-rollback.sh` 回滚。  
- 本轮未跑带 API Key 的 chat smoke（无生产 Key）。
