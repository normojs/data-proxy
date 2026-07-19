# 生产部署证据 2026-07-20：`sha-5531ece2`

## 结论

**PASS**：`https://dp.app.mbu.ltd` 已从 `sha-9de9a8d7` 升级到 **`sha-5531ece2`**（DP-3 文档收口 + DP-4 账户额度 API + DP-5 `/invitation` 页）。

## 环境

| 项 | 值 |
| --- | --- |
| 主机 | `47.122.29.88` |
| 提交 | `5531ece2` |
| Package CI | run `29700214168` success |
| 包 | `data-proxy-5531ece2-linux-amd64.tar.gz` |
| SHA256 | `5ddfcda1f60785e7dd2bbaff62001bb57d3e1842d4ab735815cb0d272a51245e` |
| 回滚归档 | `image-archive/20260719T193433Z_data-proxy_9de9a8d7.tar` |

## 步骤

1. 下载 Package 产物 → SFTP 上传  
2. `sha256sum -c` OK；`prod-deploy.sh` load `data-proxy:5531ece2`  
3. pin `docker-compose.prod.yml` image → `data-proxy:5531ece2`  
4. `force-recreate` → **healthy**

## 验收

| 检查 | 结果 |
| --- | --- |
| 容器 | `data-proxy:5531ece2` healthy |
| 本机 / 公网 version | `sha-5531ece2` |
| production smoke（无 Key） | api_status / agent install / OIDC / JWKS **passed** |
| `GET /api/usage/account` 无 Key | 非 200 成功业务体（需 Token；未授权） |
| `/invitation` | HTTP 200（SPA shell 1026B；登录后进邀请页） |

## 本版上线能力

- **DP-3**：`niaoweisi-desktop-api-reference` 明确 sk- vs 会话边界  
- **DP-4**：`GET /api/usage/account`；`GET /api/usage/token` 附带 `data.account`  
- **DP-5**：站内 `/invitation` + `/wallet#invitation`
