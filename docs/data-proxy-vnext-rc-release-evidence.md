# Data Proxy vNext RC 发布证据

日期：2026-06-26

本项目基于 `new-api`，本次 RC 继续保留 AGPLv3、`NOTICE`、attribution 和
第三方许可证文件。本文只记录可公开的发布证据，不记录 API key、Cookie、
数据库密码、证书、微信商户私钥、诊断包或 raw capture 数据。

## 范围

当前 RC 目标是单机可部署、可回滚、可诊断版本。

协议转换长尾已从当前发布线移出，后续在 vNext 稳定后再独立评审。当前本地
停车场为：

```text
stash@{0}: On main: park protocol conversion longtail after vnext
```

## Git 状态

- current local commit SHA: `87e603b87d92da02aed9f058627c3a52801550b7`
- current local commit short: `87e603b8`
- branch: `main`
- remote: `normojs/data-proxy`
- pushed ref: 待推送；上一轮已推送 RC 为 `00821cf5`

最近提交：

```text
87e603b8 chore: update ui translations for release changes
3b48f0f6 fix: clarify admin-only service status nav setting
f4ceb384 feat: add connected app management token flow
02748469 feat: add load-aware multi-key channel routing
8db0c47a feat: improve usage log filtering and exports
d8764db5 feat: add shared filter combobox and token formatter
f4d007ab chore: classify data proxy worktree changes
```

## 本地验证

已通过：

```bash
TMPDIR=/tmp scripts/data-proxy-release-gate.sh --scan-all
scripts/data-proxy-release-gate.sh --with-tests --scan-all
scripts/data-proxy-focused-regression.sh --p1
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-focused-regression.sh --p3
scripts/data-proxy-focused-regression.sh --all --frontend
```

P1/P2/P3 聚焦回归覆盖：

- P1：渠道 failover、临时熔断、用户绑定分组和 Key 限制。
- P2：request trace、诊断候选、诊断包、capture、训练数据 API。
- P3：Tunnel、MCP Gateway、`dpa`、HTTP/TCP Tunnel、Bridge policy。

工作区审计：

```bash
TMPDIR=/tmp scripts/data-proxy-worktree-audit.sh
```

结果为空，当前发布候选工作区干净；协议转换长尾不进入当前提交线。

2026-06-27 本轮本地 RC 增量：

- 使用日志：筛选下拉选项、Token 统计、详情 PNG/SVG 导出、从用户列表跳转
  common logs。
- 渠道：多 key 负载保护粘性分配、请求生命周期计数释放、dashboard
  按渠道/模型筛选。
- Connected App：`codex-dp` 管理令牌流、客户端 token 创建/轮换/吊销/分组
  更新接口。
- Topbar：服务状态入口配置页明确“无论开关普通用户都不可见”。

尚未完成的发布证据：

- 推送 `87e603b8` 到 `normojs/data-proxy`。
- 等待 GitHub CI 和 Docker package workflow 重新运行并记录 run URL。
- 生成新镜像包后部署到服务器并补生产 smoke。

## GitHub Actions

### CI

- workflow: `CI`
- run: `https://github.com/normojs/data-proxy/actions/runs/28246370371`
- conclusion: `success`
- head SHA: `00821cf5be1b07e1cff68bc7be57204d0b89027a`

已通过 jobs：

- Backend
- Frontend
- Snapless Connected App
- Fusion Benchmark

说明：上一轮 `2c564ce9` 的 CI 曾因 `docker-compose.prod.yml` 未提交失败。已在
`00821cf5` 提交中补入可公开的生产 Compose 模板，并将其纳入发布基线审计。

### Package Data Proxy Image

- workflow: `Package Data Proxy image`
- run: `https://github.com/normojs/data-proxy/actions/runs/28246370448`
- conclusion: `success`
- head SHA: `00821cf5be1b07e1cff68bc7be57204d0b89027a`

已通过步骤：

- Build linux/amd64 image
- Smoke test MySQL migration
- Export image package
- Upload image package

Artifact：

```text
data-proxy-00821cf5-linux-amd64
```

本地下载位置：

```text
/tmp/data-proxy-package-00821cf5/data-proxy-00821cf5-linux-amd64/
```

包文件：

```text
data-proxy-00821cf5-linux-amd64.tar.gz
data-proxy-00821cf5-linux-amd64.sha256
```

校验：

```text
45dcac3b1e8a5f89c4eddb221aa001a279320589f5f3b3aac6723cdd7197f9da  data-proxy-00821cf5-linux-amd64.tar.gz
```

## 部署状态

待执行。

当前阻塞点：

- `ssh root@dp.app.mbu.ltd` 使用本机默认 SSH key 被服务器拒绝。
- 本地 `http://127.0.0.1:30837/mcp` 当前没有监听，无法使用 Electerm MCP 通道。

已准备好的部署包：

```text
/tmp/data-proxy-package-00821cf5/data-proxy-00821cf5-linux-amd64/data-proxy-00821cf5-linux-amd64.tar.gz
```

恢复服务器连接后，建议执行：

```bash
cd /root/workspace/dataproxy/data-proxy
git fetch normojs main
git checkout main
git reset --hard 00821cf5be1b07e1cff68bc7be57204d0b89027a
scripts/prod-deploy.sh /root/workspace/dataproxy/data-proxy-00821cf5-linux-amd64.tar
```

如果使用本地已下载的 gzip 包，需要先在服务器解压：

```bash
gunzip -c data-proxy-00821cf5-linux-amd64.tar.gz > data-proxy-00821cf5-linux-amd64.tar
scripts/prod-deploy.sh ./data-proxy-00821cf5-linux-amd64.tar
```

部署脚本会在切换前归档当前运行镜像到：

```text
/root/workspace/dataproxy/image-archive
```

## 生产 Smoke 待办

部署后至少执行：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
scripts/data-proxy-production-smoke.sh
```

如果有可用 API key 和管理员 header，再补：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='<model>' \
DATA_PROXY_ADMIN_HEADER='<admin-header>' \
DATA_PROXY_SMOKE_DIAGNOSTIC=1 \
scripts/data-proxy-production-smoke.sh
```

生产 smoke 需要补充记录：

- 当前运行镜像 tag 或 image id。
- 当前运行镜像 digest。
- 回滚镜像归档路径。
- `/api/status` 结果。
- Chat/Responses request id。
- request trace/diagnostic bundle 结果。
- 同模型备用渠道 failover smoke 结果。
- Tunnel/`dpa status --json` 结果。
