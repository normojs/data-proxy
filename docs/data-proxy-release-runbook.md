# Data Proxy Release Runbook

本文档记录 Data Proxy 在 `normojs/data-proxy` 上的最小发布链路。它补充 GitHub CI 和 V1.3 发布证据模板，用于让源码、镜像、回滚和 new-api 开源协议合规保持可追溯。

## 发布前检查

当前单机版本发布前，先运行 Data Proxy 通用 release gate。默认模式只做非破坏性
卫生检查：工作区 artifact/密钥路径、私钥内容、new-api 许可证/NOTICE、生产
部署脚本、Docker workflow 存在性和 tracked diff 空白检查。

```bash
scripts/data-proxy-release-gate.sh
```

发布候选 commit 冻结后，建议再运行带测试的 gate：

```bash
scripts/data-proxy-release-gate.sh --with-tests
```

在有 Docker 的构建机或服务器上，可额外校验生产 compose。这个检查会同时读取
`docker-compose.prod.yml` 和 `docker-compose.wechat-pay.yml`，用于避免发布或回滚
时丢失微信支付证书挂载：

```bash
scripts/data-proxy-release-gate.sh --with-docker-config
```

如需把 token-like 字符串警告升级为失败：

```bash
DATA_PROXY_RELEASE_GATE_STRICT_SECRETS=1 scripts/data-proxy-release-gate.sh
```

1. 确认 GitHub `CI` workflow 在目标 commit 上通过，至少包含 `Backend`、`Frontend`、`Snapless Connected App` 和 `Fusion Benchmark` jobs。
2. 本地或 CI 至少覆盖：
   - `scripts/data-proxy-release-gate.sh`
   - `go test ./model ./controller ./service ./router ./oauth`
   - `cd web/default && bun run typecheck`
   - `cd web/default && bun run smoke:approval-notification-links`
   - `scripts/snapless-connected-app-preflight.sh`
   - `git diff --check`
3. 预发环境有可用账号时，运行 `make snapless-connected-app-preprod-smoke` 生成真实 request ID、app ID、token ID、device session 和 outbox 可见性记录。
4. 需要整理 Snapless Connected App 发布证据时，运行 `make snapless-connected-app-release-evidence` 生成当前 commit 的 CI、job URL、tag 和 Docker digest 快照。
5. 确认 `LICENSE`、`NOTICE`、`THIRD-PARTY-LICENSES.md` 仍随仓库和 Docker 镜像分发。
6. 确认前端可见位置仍保留原项目链接和文案：`Frontend design and development by New API contributors.`
7. 确认工作区没有本地 DB、日志、Playwright 输出、缓存、构建产物或密钥文件混入提交。

## 生产 Smoke

部署后先跑最小健康检查：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
scripts/data-proxy-production-smoke.sh
```

如果要验证 LLM 中转链路，传入临时 API key 和一个可用模型。脚本不会打印 API key，
只记录 request id 和 smoke 结果：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='gpt-4o-mini' \
DATA_PROXY_SMOKE_REQUIRE_REQUEST_ID=1 \
scripts/data-proxy-production-smoke.sh
```

如果要同时验证 request trace、diagnostic candidates 和诊断报告生成，传入管理员
认证。`DATA_PROXY_ADMIN_HEADER` 使用完整 HTTP header，例如
`Cookie: session=...`；也可以用管理员系统 access token，并同时传
`DATA_PROXY_ADMIN_USER_ID`。默认不会下载诊断 zip；需要下载时再显式开启
`DATA_PROXY_SMOKE_DOWNLOAD_BUNDLE=1`。发布验收建议开启
`DATA_PROXY_SMOKE_REQUIRE_ADMIN=1` 和 `DATA_PROXY_SMOKE_REQUIRE_REQUEST_ID=1`，
避免缺少管理员认证、缺少 request id 或 trace 不可用时被误认为完整通过。

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_SMOKE_MODEL='gpt-4o-mini' \
DATA_PROXY_ADMIN_HEADER='Cookie: session=...' \
DATA_PROXY_ADMIN_USER_ID='1' \
DATA_PROXY_SMOKE_REQUIRE_ADMIN=1 \
DATA_PROXY_SMOKE_REQUIRE_REQUEST_ID=1 \
DATA_PROXY_SMOKE_DIAGNOSTIC=1 \
scripts/data-proxy-production-smoke.sh
```

管理员 access token 方式：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_SMOKE_REQUEST_ID='REQ_ID' \
DATA_PROXY_ADMIN_ACCESS_TOKEN='***' \
DATA_PROXY_ADMIN_USER_ID='1' \
DATA_PROXY_SMOKE_REQUIRE_ADMIN=1 \
DATA_PROXY_SMOKE_REQUIRE_REQUEST_ID=1 \
DATA_PROXY_SMOKE_DIAGNOSTIC=1 \
DATA_PROXY_SMOKE_CHAT=0 \
DATA_PROXY_SMOKE_RESPONSES=0 \
scripts/data-proxy-production-smoke.sh
```

也可以只验证某个已知 request id 的 trace/diagnostic：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_ADMIN_HEADER='Cookie: session=...' \
DATA_PROXY_ADMIN_USER_ID='1' \
DATA_PROXY_SMOKE_REQUEST_ID='REQ_ID' \
DATA_PROXY_SMOKE_REQUIRE_ADMIN=1 \
DATA_PROXY_SMOKE_REQUIRE_REQUEST_ID=1 \
DATA_PROXY_SMOKE_DIAGNOSTIC=1 \
DATA_PROXY_SMOKE_CHAT=0 \
DATA_PROXY_SMOKE_RESPONSES=0 \
scripts/data-proxy-production-smoke.sh
```

### 同模型坏渠道自动切备用 smoke

同模型 failover 生产验证使用非破坏式脚本。脚本只发送请求并读取 request trace /
diagnostic candidates，不会创建、修改、禁用或删除生产渠道。运行前需要管理员先在
控制台准备一组一次性测试渠道：

- 两个启用渠道服务同一个测试模型名；
- 第一条渠道稳定返回临时故障，例如 502/503/429 或超时；
- 第二条渠道可正常返回；
- `Retry Times >= 1`；
- 临时故障规则包含该故障状态码或关键字，例如 `429,500-599`；
- 不要把 5xx 放进硬故障自动禁用规则，避免测试渠道被持久禁用。

运行示例：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_API_KEY='sk-***' \
DATA_PROXY_FAILOVER_MODEL='deepseek-ai/DeepSeek-V4-Flash' \
DATA_PROXY_ADMIN_HEADER='Cookie: session=...' \
DATA_PROXY_ADMIN_USER_ID='1' \
DATA_PROXY_FAILOVER_EXPECT_FAILED_STATUS_CODE=502 \
DATA_PROXY_FAILOVER_OUTPUT=/tmp/data-proxy-failover-smoke.md \
scripts/data-proxy-channel-failover-smoke.sh
```

如果已经有一条已知 request id，也可以只验证 trace：

```bash
DATA_PROXY_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_FAILOVER_REQUEST_ID='REQ_ID' \
DATA_PROXY_ADMIN_ACCESS_TOKEN='***' \
DATA_PROXY_ADMIN_USER_ID='1' \
scripts/data-proxy-channel-failover-smoke.sh
```

通过条件：

- 请求最终有 consume 日志；
- request trace 中存在 `admin_info.channel_failover`；
- 至少一个 `failed` 事件包含 `retry_planned=true`；
- 至少一个后续 `selected` 事件的 `retry_index > 0`；
- `selected` 事件中出现至少两个不同渠道 ID；
- diagnostic candidates 能按 `source=failover` 找到该 request id。

每次生产 smoke 至少把 summary、Docker image digest、tag、commit SHA、回滚镜像
记录到发布证据里。不要把 API key、Cookie、诊断 zip 或原始 capture bundle
提交到 Git。

## 版本和镜像

标准版本 tag 建议使用：

```bash
git tag v1.3.0
git push normojs v1.3.0
```

tag push 会触发 `Publish Data Proxy image` workflow，并发布到 GitHub Container Registry：

```text
ghcr.io/normojs/data-proxy:v1.3.0
ghcr.io/normojs/data-proxy:sha-<short-sha>
ghcr.io/normojs/data-proxy:latest
```

也可以在 GitHub Actions 页面手动触发 `Publish Data Proxy image`，输入一个已存在的 tag。手动触发默认不更新 `latest`，除非显式勾选 `publish_latest`。

## dpa Agent 发布资产

`Data Proxy Agent` workflow 会在 tag 发布时构建 `dpa` 的跨平台资产：

- Linux/macOS/Windows 的 `amd64` 和 `arm64` tar/zip。
- Linux `deb` / `rpm`。
- Windows MSI。
- Homebrew formula。
- `data-proxy-agent-manifest.json` 机器可读更新清单。
- `checksums.txt` 及 cosign 签名。

manifest 会优先引用已完成 notarization 的 macOS archive；如果签名证书未配置，
则回退到普通 macOS archive。控制台或服务器可以缓存这些 release asset，然后把
manifest 代理到 Data Proxy 自己的域名：

```bash
dpa update --manifest-url https://dp.app.mbu.ltd/agent/releases/data-proxy-agent-manifest.json --dry-run
dpa update --manifest-url https://dp.app.mbu.ltd/agent/releases/data-proxy-agent-manifest.json
```

首次安装也可以复用同一个 manifest，避免脚本访问 GitHub Release API：

```bash
curl -fsSL https://dp.app.mbu.ltd/agent/install-data-proxy-agent.sh | \
  DATA_PROXY_AGENT_MANIFEST_URL=https://dp.app.mbu.ltd/agent/releases/data-proxy-agent-manifest.json sh
```

发布前需要确认 manifest 和 checksum 一起上传；如果使用自建 CDN 或对象存储镜像
release asset，需要设置相同文件名并保持 manifest 中的 URL 与 sha256 对应。

### dpa status smoke

部署或升级 `dpa` 后，先跑非破坏式状态 smoke。脚本会生成临时配置，不读取或修改
本机真实 agent 配置，也不会打印 token：

```bash
DATA_PROXY_AGENT_BIN=/usr/local/bin/dpa \
DATA_PROXY_AGENT_SMOKE_BASE_URL=https://dp.app.mbu.ltd \
scripts/data-proxy-agent-status-smoke.sh
```

如需同时验证本地诊断 JSON，可加 `DATA_PROXY_AGENT_SMOKE_DOCTOR=1`：

```bash
DATA_PROXY_AGENT_BIN=/usr/local/bin/dpa \
DATA_PROXY_AGENT_SMOKE_BASE_URL=https://dp.app.mbu.ltd \
DATA_PROXY_AGENT_SMOKE_DOCTOR=1 \
scripts/data-proxy-agent-status-smoke.sh
```

通过条件：

- `dpa status --json` 返回合法 JSON；
- JSON 中 `config_loaded=true`、`token_configured=true`；
- MCP/HTTP/TCP route 计数和 capabilities 可见；
- 输出不能包含临时 token 明文；
- 开启 doctor smoke 时，`doctor --json` 至少返回 workspace 和 local audit 诊断项。

## 发布证据

每次发布至少记录：

- commit SHA
- git tag
- CI run URL
- Docker workflow run URL
- 镜像 tag
- 镜像 digest
- 数据库迁移说明
- 站内通知、邮件、webhook、失败重试和关闭开关的验证结果
- 回滚负责人和回滚窗口

V1.3 通知闭环的业务证据继续补到 `docs/enterprise-governance-v1.3-release-evidence.md`；Snapless Connected App 证据补到 `docs/snapless-connected-app-v1.3-release-evidence.md` 或对应变更单。

可用以下命令在预发环境执行 Snapless Connected App smoke。`*_HEADER` 使用完整 HTTP header，例如 `Cookie: session=...` 或 `Authorization: Bearer ...`；脚本不会打印 API key 明文。默认保留测试 app 便于排查和截图；如需执行后停用测试 app，设置 `SNAPLESS_PREPROD_CLEANUP=1`。

```bash
DATA_PROXY_BASE_URL=https://preprod.example.com \
ADMIN_HEADER='Cookie: session=...' \
DEVELOPER_HEADER='Cookie: session=...' \
AUTHORIZING_USER_HEADER='Cookie: session=...' \
SNAPLESS_PREPROD_CLEANUP=1 \
SNAPLESS_PREPROD_CONFIRM=1 \
make snapless-connected-app-preprod-smoke
```

可用以下命令生成 Snapless Connected App 的发布证据快照：

```bash
make snapless-connected-app-release-evidence
```

### V1.3.0 发布证据

- commit SHA: `b1e14edab6cb0e0628141359e4f27e376f55c165`
- git tag: `v1.3.0`
- CI run: `https://github.com/normojs/data-proxy/actions/runs/27858373811`
- Docker workflow run: `https://github.com/normojs/data-proxy/actions/runs/27858433012`
- Docker image: `ghcr.io/normojs/data-proxy`
- Published tags:
  - `ghcr.io/normojs/data-proxy:v1.3.0`
  - `ghcr.io/normojs/data-proxy:sha-b1e14edab6cb`
  - `ghcr.io/normojs/data-proxy:latest`
- Image digest: `sha256:7650bff674c4a2b070197feba382414c47285de0578ddb2749dbbb84996046ac`
- Database migration note: V1.3 uses existing GORM auto-migration paths; no one-off SQL rollback script was required for the recorded image build.
- Compliance note: Docker image build keeps `LICENSE`, `NOTICE` and `THIRD-PARTY-LICENSES.md` under `/licenses/`.

## 回滚

如果服务器使用本地 tar 包部署，先使用生产脚本回滚到最新的本地镜像归档。这个脚本会继续带上
`docker-compose.wechat-pay.yml`，避免回滚后微信支付证书挂载丢失：

```bash
scripts/prod-rollback.sh
```

也可以指定某个历史归档：

```bash
scripts/prod-rollback.sh /root/workspace/dataproxy/image-archive/<archive>.tar
```

发布新镜像时使用生产部署脚本，它会在切换前保存当前运行镜像，默认保留最近 10 份：

```bash
scripts/prod-deploy.sh ./data-proxy-<tag>.tar
scripts/prod-deploy.sh ghcr.io/normojs/data-proxy:<tag>
```

如果使用镜像仓库，仍然优先记录并回滚到上一个已验证镜像 digest，而不是仅依赖浮动 tag：

```bash
docker pull ghcr.io/normojs/data-proxy@sha256:<previous-digest>
scripts/prod-rollback.sh ghcr.io/normojs/data-proxy@sha256:<previous-digest>
```

回滚后需要验证：

- 服务启动成功。
- `scripts/prod-compose.sh config` 仍包含 `docker-compose.wechat-pay.yml` 中的 `/run/secrets/data-proxy/wechatpay:ro` 挂载。
- 数据库迁移没有造成不可逆阻塞；如果有迁移风险，发布前必须准备回滚 SQL 或只读降级方案。
- 企业治理开关可关闭外部 email/webhook 投递。
- V1.3 站内通知仍可读取，审批/审计 deep link 不报错。

## 合规注意事项

- Data Proxy 基于 new-api，必须继续遵守 AGPLv3 和 `NOTICE` 中的 Section 7 附加要求。
- Docker 镜像必须保留 `/licenses/LICENSE`、`/licenses/NOTICE` 和 `/licenses/THIRD-PARTY-LICENSES.md`。
- 修改品牌、页脚、关于页或法律页时，不能移除原项目链接和 attribution 文案。
