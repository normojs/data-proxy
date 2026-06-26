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

- commit SHA: `00821cf5be1b07e1cff68bc7be57204d0b89027a`
- commit short: `00821cf5`
- branch: `main`
- remote: `normojs/data-proxy`
- pushed ref: `refs/heads/main`

最近提交：

```text
00821cf5 chore: track production compose template
2c564ce9 docs: plan vnext stabilization tasks
5824918e docs: record focused regression status
```

## 本地验证

已通过：

```bash
TMPDIR=/tmp scripts/data-proxy-release-gate.sh --scan-all
scripts/data-proxy-focused-regression.sh --p1
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-focused-regression.sh --p3
```

P1/P2/P3 聚焦回归覆盖：

- P1：渠道 failover、临时熔断、用户绑定分组和 Key 限制。
- P2：request trace、诊断候选、诊断包、capture、训练数据 API。
- P3：Tunnel、MCP Gateway、`dpa`、HTTP/TCP Tunnel、Bridge policy。

工作区审计：

```bash
TMPDIR=/tmp scripts/data-proxy-worktree-audit.sh
```

结果为空，当前发布候选工作区干净；协议转换长尾保存在 stash，不在当前提交中。

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
