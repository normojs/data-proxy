# Data Proxy vNext RC 发布证据

日期：2026-06-27

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

- current local commit SHA: `45eb77749fb5173649a4ec35527fbbe2ee6ee2d1`
- current local commit short: `45eb7774`
- branch: `main`
- remote: `normojs/data-proxy`
- pushed ref: `normojs/main`

最近提交：

```text
45eb7774 docs: record current release validation status
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
scripts/data-proxy-release-gate.sh --scan-all
scripts/data-proxy-release-gate.sh --with-tests --scan-all
scripts/data-proxy-focused-regression.sh --all --frontend
```

P1/P2/P3 聚焦回归覆盖：

- P1：渠道 failover、临时熔断、用户绑定分组和 Key 限制。
- P2：request trace、诊断候选、诊断包、capture、训练数据 API。
- P3：Tunnel、MCP Gateway、`dpa`、HTTP/TCP Tunnel、Bridge policy。

工作区审计：

```bash
TMPDIR=/tmp scripts/data-proxy-worktree-audit.sh
git status --short
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

## GitHub Actions

### CI

- workflow: `CI`
- run: `https://github.com/normojs/data-proxy/actions/runs/28285624221`
- conclusion: `success`
- head SHA: `45eb77749fb5173649a4ec35527fbbe2ee6ee2d1`
- completed at: `2026-06-27T09:50:18Z`

### Package Data Proxy Image

- workflow: `Package Data Proxy image`
- run: `https://github.com/normojs/data-proxy/actions/runs/28285624210`
- conclusion: `success`
- head SHA: `45eb77749fb5173649a4ec35527fbbe2ee6ee2d1`
- completed at: `2026-06-27T09:52:02Z`

GitHub package workflow 已通过。由于本次服务器侧 GitHub 下载速度不稳定，
实际部署使用本地构建并通过 Electerm SFTP 上传的等价 linux/amd64 镜像包。

本地部署包：

```text
/tmp/data-proxy-45eb7774-linux-amd64.tar.gz
/tmp/data-proxy-45eb7774-linux-amd64.sha256
```

sha256：

```text
9ad97123f64e8e5b3bb37aed38cfa9831fb0b1f808f2dbc8918457072eeaf532
```

## 部署状态

状态：已部署到生产服务器。

部署方式：

- 使用 `data-proxy-local-deploy` 流程。
- 通过 Electerm SFTP 上传镜像包、sha256 文件和一次性远端部署脚本。
- 远端部署脚本先校验 sha256，再归档旧镜像，最后加载并切换
  `docker-compose.prod.yml` 中的镜像 tag。
- 启动命令包含 WeChat Pay override：
  `docker compose -f docker-compose.prod.yml -f docker-compose.wechat-pay.yml up -d data-proxy`。

当前线上镜像：

```text
data-proxy:45eb7774
```

远端镜像信息：

```text
id=sha256:4c086fbd7210e5dd4d776aeeaf5720a383bf2f0d88cb24ef9110ce95aaa3493f
created=2026-06-27T10:17:48.891298262Z
```

容器状态：

```text
data-proxy data-proxy:45eb7774 Up 8 minutes (healthy) 3000/tcp, 127.0.0.1:13002->13002/tcp
```

Compose 镜像行：

```text
docker-compose.prod.yml: image: data-proxy:45eb7774
```

回滚归档：

```text
/root/workspace/dataproxy/image-archive/20260627-182613_data-proxy_70800ed0.tar
/root/workspace/dataproxy/image-archive/20260627-182613_data-proxy_70800ed0.tar.meta
```

回滚提示保存在 `.tar.meta` 中；如需回滚，先 `docker load` 该归档，再把
`docker-compose.prod.yml` 镜像行切回旧 tag 并重新 `docker compose up -d`。

## 生产 Smoke

### `/api/status`

本地端口：

```text
curl http://127.0.0.1:13002/api/status
success=true
version=sha-45eb7774
server_address=https://dp.app.mbu.ltd
```

公网：

```text
curl -k https://dp.app.mbu.ltd/api/status
success=true
version=sha-45eb7774
server_address=https://dp.app.mbu.ltd
```

### Chat / Responses

使用生产 URL 和国产模型 key 执行：

```bash
DATA_PROXY_BASE_URL='https://dp.app.mbu.ltd' \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='deepseek-ai/DeepSeek-V4-Flash' \
DATA_PROXY_SMOKE_TIMEOUT_SECONDS=60 \
DATA_PROXY_SMOKE_OUTPUT='/tmp/data-proxy-smoke-45eb7774-domestic.md' \
scripts/data-proxy-production-smoke.sh
```

结果：

```text
api_status=passed
chat_completions=passed
responses=passed
completed_at_utc=2026-06-27T10:29:47Z
```

手动补充 request id：

```text
chat_request_id=202606271030428481627518268d9d6XaLnnOAZ
responses_request_id=20260627103047195310618268d9d6FXosDTVU
```

### 待管理员登录态补测

以下接口需要管理员登录态或用户系统 access token，并且需要同时携带
`New-Api-User`；普通 OpenAI API key 不能访问控制台诊断接口。

待补测项目：

- request trace：`GET /api/log/request/:request_id`
- diagnostic candidates：`GET /api/log/request-diagnostic-candidates`
- diagnostic report：`POST /api/log/request/:request_id/diagnostic`
- diagnostic bundle：`GET /api/log/request/:request_id/diagnostic/bundle`
- common logs request id 快捷入口和详情 PNG/SVG 导出 UI。
- 同模型坏渠道自动切备用的生产演练。
- Tunnel / `dpa status --json` 生产演练。

本地回归已经覆盖上述能力的代码路径；生产补测需要在管理员控制台会话中执行，
或提供只用于 smoke 的临时管理员 access token 和对应 `New-Api-User`。
