# 生产部署证据 2026-07-20：`sha-9de9a8d7`

## 结论

**PASS**：`https://dp.app.mbu.ltd` 已从 `sha-e1e59279` 升级到 **`sha-9de9a8d7`**（commit `9de9a8d7`：DP-1 Device poll 用户摘要 + DP-2 `signup_app` 归因路径）。

## 环境

| 项 | 值 |
| --- | --- |
| 主机 | `47.122.29.88`（snsc-prod-应用2） |
| 目录 | `/root/workspace/dataproxy/data-proxy` |
| Git | `9de9a8d7`（已 `git push normojs main`，Everything up-to-date） |
| Package CI | run `29697784348` success |
| 包 | `data-proxy-9de9a8d7-linux-amd64.tar.gz` |
| SHA256 | `0d1a21ef43b8ca1536bc062e06baa687c93b39681653de73d886d5bcaa91049d` |
| 回滚归档 | `image-archive/20260719T183343Z_data-proxy_e1e59279.tar` |

## 步骤

1. `gh run download` 获取产物（经本地代理）  
2. SFTP 上传 tar.gz / sha256 / `data-proxy-remote-deploy-9de9a8d7.sh`  
3. `sha256sum -c` OK  
4. `prod-deploy.sh` load `data-proxy:9de9a8d7`，归档上一版 `e1e59279`  
5. 将服务器 `docker-compose.prod.yml` 的硬编码 `image:` 改为 `data-proxy:9de9a8d7`（保留 `.bak.*`）  
6. `prod-compose.sh up -d --force-recreate --no-deps data-proxy`  
7. 容器 **healthy**

## 验收

| 检查 | 结果 |
| --- | --- |
| 容器 | `data-proxy:9de9a8d7` healthy |
| 本机 `/api/status` | `success=true`，`version=sha-9de9a8d7` |
| 公网 `x-new-api-version` | `sha-9de9a8d7` |
| 公网 `/api/status` | `sha-9de9a8d7` |
| production smoke（无 Key） | `api_status` / agent install / OIDC discovery / JWKS = passed；chat 跳过 |

## 本版业务变更（随镜像上线）

- **DP-1**：Device poll 成功载荷含 `user.id` / `username` / `display_name` / `group`  
- **DP-2**：`verification_uri` 与 `/connect/device` → sign-in/sign-up 链路保留 `signup_app`

## 备注

- 生产 compose 仍用硬编码 image tag；下次部署需继续改 pin 或改回 `${DATA_PROXY_IMAGE}`。  
- 未跑带 `sk-` 的 chat smoke（无生产 API Key）。
