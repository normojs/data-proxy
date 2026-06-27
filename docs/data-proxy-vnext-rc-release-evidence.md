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

- current local commit SHA: `f1526cadb990ca770f19a82538f10e365effc974`
- current local commit short: `f1526cad`
- branch: `main`
- remote: `normojs/data-proxy`
- pushed ref: `normojs/main`

最近提交：

```text
f1526cad fix: capture lowercase request id headers in smoke scripts
419d5e6d feat: clarify channel failover retry settings
89a14252 ci: wire dpa status smoke into agent checks
2112c11d test: add dpa status smoke
654649f0 ci: check usage log detail export smoke
023475fd chore: classify usage log export smoke
2d8be136 test: cover usage log detail export affordances
50cfb11c chore: classify failover smoke artifacts
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
- 生产 smoke 脚本：支持管理员系统 access token + `New-Api-User` 方式验证
  request trace、diagnostic candidates、诊断报告和诊断包下载。
- 渠道故障切换设置：管理端文案明确 `RetryTimes >= 1`、临时故障、硬故障、
  熔断窗口、冷却时间和 request trace 观测关系。
- 用户绑定分组：补充服务端回归，覆盖修改 Key 到不可用分组被拒绝，以及旧
  Key 绑定分组失效后在 `TokenAuth` 请求入口被 403 拦截。

2026-06-27 本轮增量验证：

```bash
go test ./controller -run 'Test(UpdateTokenRejectsBoundUnavailableGroup|AddTokenRejectsBoundUnavailableGroup|GetAllTokensAnnotatesUnavailableGroup|TokenGroupUnavailableReasonHonorsUserBindings)' -count=1
go test ./middleware -run 'TestTokenAuthRejectsLegacyTokenOutsideUserBoundGroups' -count=1
scripts/data-proxy-focused-regression.sh --p1 --frontend
scripts/data-proxy-release-gate.sh --scan-all
```

结果：全部通过；工作区重新收口为空。

2026-06-27 smoke request id 解析修复验证：

```bash
bash -n scripts/data-proxy-production-smoke.sh scripts/data-proxy-channel-failover-smoke.sh
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='***' \
DATA_PROXY_SMOKE_MODEL='deepseek-ai/DeepSeek-V4-Flash' \
scripts/data-proxy-production-smoke.sh
scripts/data-proxy-focused-regression.sh --p2
scripts/data-proxy-release-gate.sh --scan-all
scripts/data-proxy-worktree-audit.sh
```

结果：

```text
api_status=passed
chat_completions=passed
chat_request_id=202606271653506232905768268d9d67zmrUET1
responses=passed
responses_request_id=202606271653518524336168268d9d6SB8Qlcm3
diagnostic_candidates=skipped_no_admin_auth
request_trace=skipped_no_admin_auth
P2 focused regression=passed
release gate=passed
```

说明：生产服务返回的小写 `x-oneapi-request-id` 已能被生产 smoke 和同模型
failover smoke 统一识别。该提交只修改本地验证脚本，不改变线上运行时行为。

## GitHub Actions

### CI

- workflow: `CI`
- run: `https://github.com/normojs/data-proxy/actions/runs/28295743276`
- conclusion: `success`
- head SHA: `f1526cadb990ca770f19a82538f10e365effc974`
- completed at: `2026-06-27T17:00:39Z`

### Data Proxy Agent

- workflow: `Data Proxy Agent`
- run: `https://github.com/normojs/data-proxy/actions/runs/28287148059`
- conclusion: `success`
- head SHA: `ba482284361e4bf2b4004b6061178ed2949dcd8d`
- completed at: `2026-06-27T10:58:36Z`

本轮 `baf94f18` 未触发新的 Data Proxy Agent workflow；最近一次 agent
workflow 仍为上述成功记录。

### Package Data Proxy Image

- workflow: `Package Data Proxy image`
- run: `https://github.com/normojs/data-proxy/actions/runs/28295743275`
- conclusion: `success`
- head SHA: `f1526cadb990ca770f19a82538f10e365effc974`
- completed at: `2026-06-27T17:01:41Z`

GitHub package workflow 已通过。由于服务器侧 GitHub 下载速度不稳定，实际部署
继续使用本地构建并通过 Electerm SFTP 上传的等价 linux/amd64 镜像包。

本地部署包：

```text
/tmp/data-proxy-baf94f18-local-linux-amd64.tar.gz
/tmp/data-proxy-baf94f18-local-linux-amd64.sha256
```

sha256：

```text
3bf073f61105dfe0406053a06f48eae50f7e2ba9d23a3f39171aecf68d32a376
```

说明：标准 Dockerfile 构建在容器内 `go mod download` 阶段长时间无输出，本轮
改用本地前端构建 + 本地 linux/amd64 Go 交叉编译 + 仅 runtime Dockerfile
打包。镜像仍包含 `/licenses/LICENSE`、`/licenses/NOTICE` 和
`/licenses/THIRD-PARTY-LICENSES.md`。

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
data-proxy:baf94f18
```

远端镜像信息：

```text
id=sha256:b33450ff27d2e8dd9728af756c772f8f45ec5248028e143d9c3e3bdf21f46b8a
created=2026-06-27T12:08:38.290379964Z
```

容器状态：

```text
data-proxy data-proxy:baf94f18 Up About a minute (healthy) 3000/tcp, 127.0.0.1:13002->13002/tcp
```

Compose 镜像行：

```text
docker-compose.prod.yml: image: data-proxy:baf94f18
```

回滚归档：

```text
/root/workspace/dataproxy/image-archive/20260627-201659_data-proxy_45eb7774.tar
/root/workspace/dataproxy/image-archive/20260627-201659_data-proxy_45eb7774.tar.meta
```

回滚提示保存在 `.tar.meta` 中；如需回滚，先 `docker load` 该归档，再把
`docker-compose.prod.yml` 镜像行切回旧 tag 并重新 `docker compose up -d`。

## 生产 Smoke

### `/api/status`

本地端口：

```text
curl http://127.0.0.1:13002/api/status
success=true
version=sha-baf94f18
server_address=https://dp.app.mbu.ltd
```

公网：

```text
curl -k https://dp.app.mbu.ltd/api/status
success=true
version=sha-baf94f18
server_address=https://dp.app.mbu.ltd
```

### Chat / Responses

使用生产 URL 和国产模型 key 执行：

```bash
DATA_PROXY_BASE_URL='https://dp.app.mbu.ltd' \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='deepseek-ai/DeepSeek-V4-Flash' \
DATA_PROXY_SMOKE_TIMEOUT_SECONDS=60 \
DATA_PROXY_SMOKE_OUTPUT='/tmp/data-proxy-smoke-baf94f18-domestic.md' \
scripts/data-proxy-production-smoke.sh
```

结果：

```text
api_status=passed
chat_completions=passed
responses=passed
completed_at_utc=2026-06-27T12:20:06Z
```

手动补充 request id：

```text
chat_request_id=202606271220437615844868268d9d6zxJ9DZGp
responses_request_id=202606271220479251847698268d9d6F87OTE9R
```

### 待管理员登录态补测

以下接口需要管理员登录态或用户系统 access token，并且需要同时携带
`New-Api-User`；普通 OpenAI API key 不能访问控制台诊断接口。`ba482284`
已让 `scripts/data-proxy-production-smoke.sh` 支持
`DATA_PROXY_ADMIN_ACCESS_TOKEN` + `DATA_PROXY_ADMIN_USER_ID`。

待补测项目：

- request trace：`GET /api/log/request/:request_id`
- diagnostic candidates：`GET /api/log/request-diagnostic-candidates`
- diagnostic report：`POST /api/log/request/:request_id/diagnostic`
- diagnostic bundle：`GET /api/log/request/:request_id/diagnostic/bundle`
- common logs request id 快捷入口和详情 PNG/SVG 导出 UI。
- 同模型坏渠道自动切备用的生产演练。
- Tunnel / `dpa status --json` 生产演练。

本次未直接在服务器生成临时管理员 access token，原因：

- `data-proxy` 容器和宿主机没有现成 `mysql` / `mariadb` 客户端。
- 宿主机 Python 没有 MySQL 驱动。
- 尝试拉取临时工具镜像受 Docker Hub 网络超时影响失败。
- 为避免直接改线上管理员账号凭据，本轮不通过数据库写入方式生成 token。

可执行补测命令：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_ADMIN_ACCESS_TOKEN='***' \
DATA_PROXY_ADMIN_USER_ID='1' \
DATA_PROXY_SMOKE_REQUEST_ID='202606271220479251847698268d9d6F87OTE9R' \
DATA_PROXY_SMOKE_DIAGNOSTIC=1 \
DATA_PROXY_SMOKE_DOWNLOAD_BUNDLE=1 \
DATA_PROXY_SMOKE_CHAT=0 \
DATA_PROXY_SMOKE_RESPONSES=0 \
scripts/data-proxy-production-smoke.sh
```

本地回归已经覆盖上述能力的代码路径；生产补测需要在管理员控制台会话中执行，
或提供只用于 smoke 的临时管理员 access token 和对应 `New-Api-User`。
