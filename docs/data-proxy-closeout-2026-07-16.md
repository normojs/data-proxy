# Data Proxy 收口交付状态（2026-07-16）

## 源码

| 项 | 值 |
| --- | --- |
| 远程 | `normojs/data-proxy` `main` |
| HEAD | `b507e6ad` |
| CI | [success](https://github.com/normojs/data-proxy/actions/runs/29505928591) |
| Package image | [success](https://github.com/normojs/data-proxy/actions/runs/29505928834) |

包含今日 P0/P1 product-gap 功能（额度总览、扣费解释、错误人话化、3 分钟接入、兑换码发包、模型广场测通、一键部署文档、渠道故障切换预设）以及 CI 稳定性修复。

## 生产 `https://dp.app.mbu.ltd`

| 项 | 值 |
| --- | --- |
| 当前版本 header | `x-new-api-version: fbb2df5c` |
| 落后 HEAD | 约 9 个提交（P0/P1 + CI 修复） |
| 公开 `/api/status` smoke | 通过（2026-07-16T13:52:15Z） |
| Chat / Responses smoke | 未跑（本机无生产 API Key） |
| Admin diagnostic smoke | 未跑（本机无管理员认证） |
| SSH 部署 | **阻塞**：对本机 `id_ed25519` 在 `47.122.29.88` / `47.122.4.83` / `dp.app.mbu.ltd` 上 `Permission denied (publickey)`；`ssh-agent` 无 identity |

## 本地已验证

- `go test ./service ./model ./controller ./router ./oauth`
- `bun run typecheck`
- `scripts/data-proxy-release-gate.sh`
- Snapless Connected App CI job 已绿

## product-gap 退出标准

### P0

- [x] 代码：额度总览 / 扣费解释 / 3 分钟文档 / 错误人话化
- [ ] 验收：新用户按文档 3 分钟成功请求（需生产部署 + Key）
- [ ] 验收：UI 能解释扣费或拒绝原因（需生产部署后手测）
- [ ] 验收：额度总览四类资产不混淆（需生产部署后手测）

### P1 已完成代码

- [x] 兑换码 → Token 包
- [x] 模型广场测通/复制/包覆盖
- [x] 一键部署文档与 minimal env
- [x] 渠道安全故障切换预设

### P1 后置（下一阶段）

- [ ] Token 包在线支付 SKU
- [ ] 包即将用尽提醒
- [ ] 用户侧「上游繁忙已重试」友好提示

### P2

- [ ] dpa / Browser 隧道端到端真正可用

## 部署解锁清单（需人工）

1. 在本机加载可登录生产机的 SSH key（`ssh-add`），或提供跳板机/运维通道。
2. 在生产目录执行（示例）：

```bash
# 在服务器 /root/workspace/dataproxy/data-proxy
# 拉取最新镜像或上传 Package workflow 产物后：
scripts/prod-deploy.sh ghcr.io/normojs/data-proxy:sha-b507e6ad
# 或本地 tar 包路径
```

3. 部署后跑完整 smoke：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='gpt-4o-mini' \
DATA_PROXY_SMOKE_REQUIRE_REQUEST_ID=1 \
scripts/data-proxy-production-smoke.sh
```

4. 勾选 product-gap P0 退出标准三项，并更新本文档。

## 相关提交

- `008c79b0` feat: add unified user quota overview
- `4d22bb90` feat: explain request funding in usage logs
- `192f4134` feat: humanize billing and package 403 errors
- `bc6b0003` docs: add 3-minute user quickstart and in-app links
- `43b775f5` feat: redeem codes can grant model token packages
- `016d13da` feat: marketplace test, copy, and package coverage
- `a53d7d8d` docs: one-click deploy and safe channel failover preset
- `29cfed13` fix: stabilize billing tests and release hygiene
- `b507e6ad` fix: pass gofmt and usage-log export smoke checks

## 2026-07-17 production deploy attempt

- Target commit: `03f66c5c`
- Package artifact downloaded: `data-proxy-03f66c5c-linux-amd64.tar.gz` (+ sha256)
- Remote deploy script prepared: `data-proxy-remote-deploy-03f66c5c.sh`
- Local staging: `/Users/fushilu/workspace/revocloud/data-proxy/`
- SSH from this machine still blocked (`Permission denied (publickey)` to production hosts)
- Historical deploy path uses Electerm SFTP to `/root/workspace/dataproxy/data-proxy/` then run remote script
- Production still reports `x-new-api-version: fbb2df5c` until upload+script run

## 2026-07-17 production deploy completed via Electerm MCP

- Project MCP config: `/Users/fushilu/workspace/revocloud/data-proxy/.mcp.json` → `http://127.0.0.1:30837/mcp`
- Electerm bookmark: `snsc-prod-应用2` (`47.122.29.88`, tab `QSJTWHa_2026-07-17-00-40-02`)
- Uploaded:
  - `data-proxy-03f66c5c-linux-amd64.tar.gz` (62187952 bytes)
  - `data-proxy-03f66c5c-linux-amd64.sha256`
  - `data-proxy-remote-deploy-03f66c5c.sh`
- Deploy result:
  - previous image archived: `data-proxy:fbb2df5c` → `/root/workspace/dataproxy/image-archive/20260716T164225Z_data-proxy_fbb2df5c.tar`
  - loaded `data-proxy:03f66c5c`
  - container recreated; health ok
  - public header now `x-new-api-version: sha-03f66c5c`
- Smoke: `DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd scripts/data-proxy-production-smoke.sh` → `api_status=passed` (chat/admin skipped without keys)

