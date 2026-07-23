# 生产部署证据 2026-07-23：`sha-fdf8fd73`

## 结论

**PASS**：`https://dp.app.mbu.ltd` 已从 `sha-5531ece2` 升级到 **`sha-fdf8fd73`**（邀请页 i18n + Connected App 可配置 `default_token_group`）。

## 环境

| 项 | 值 |
| --- | --- |
| 主机 | `47.122.29.88`（snsc-prod-应用2） |
| 目录 | `/root/workspace/dataproxy/data-proxy` |
| 提交 | `fdf8fd73` |
| Package CI | run `29966836973` success |
| 包 | `data-proxy-fdf8fd73-linux-amd64.tar.gz` |
| SHA256 | `97006d3f896facc839b3c44cd1e1f7f5aae32a889bf0e8134855f1b0b6838638` |
| 镜像 | `data-proxy:fdf8fd73` (`sha256:b7a44c43cd63360bb840c7519790a9e2b1414fb046f377283f9ec311bd0245a2`) |
| 回滚归档 | `image-archive/20260723T003228Z_data-proxy_5531ece2.tar` |

## 步骤

1. `gh run download 29966836973 --repo normojs/data-proxy` 获取 Package 产物  
2. Electerm SFTP 上传 tar.gz / sha256 / `data-proxy-remote-deploy-fdf8fd73.sh`  
3. `sha256sum -c` OK；归档当前镜像 `data-proxy:5531ece2`  
4. `docker load` → pin `docker-compose.prod.yml` image 为 `data-proxy:fdf8fd73`  
5. `docker compose -f docker-compose.prod.yml -f docker-compose.wechat-pay.yml up -d --force-recreate data-proxy`  
6. 容器 **healthy**，本机 `/api/status` → `version=sha-fdf8fd73`

## 验收

| 检查 | 结果 |
| --- | --- |
| 容器 | `data-proxy:fdf8fd73` healthy |
| 本机 `/api/status` | `success=true`，`version=sha-fdf8fd73` |
| 公网 `x-new-api-version` | `sha-fdf8fd73` |
| 公网 `/api/status` | `success=true`，`version=sha-fdf8fd73` |
| production smoke（无 Key） | api_status / agent install* / OIDC / JWKS **passed** |
| `/agent/install.sh` | 200 shell（`text/x-shellscript`，非 SPA） |
| `/.well-known/openid-configuration` | 200 JSON，`issuer=https://dp.app.mbu.ltd` |
| `/oauth/jwks.json` | 200 JSON |
| `/invitation` | HTTP 200（SPA shell） |

## 本版上线能力

- **i18n**：邀请页翻译 + 侧栏入口  
- **IdP**：Connected App `default_token_group` 可配置（管理端创建/更新 UI；niaoweisi 默认「鸟维斯」空值回填）

## 运维备注

- 生产 compose 仍硬编码 `image: data-proxy:<sha>`，部署脚本会 pin 到新 tag。  
- 旧镜像 `5531ece2` 已归档，可用 `scripts/prod-rollback.sh` 或手动 load 归档 tar 回滚。  
- 本轮未跑带 API Key 的 chat smoke（无生产 Key）。  
- Package 成功但同提交 CI workflow 失败（历史常态，未阻塞镜像包产物）。
