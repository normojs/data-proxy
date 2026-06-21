# Data Proxy Release Runbook

本文档记录 Data Proxy 在 `normojs/data-proxy` 上的最小发布链路。它补充 GitHub CI 和 V1.3 发布证据模板，用于让源码、镜像、回滚和 new-api 开源协议合规保持可追溯。

## 发布前检查

1. 确认 GitHub `CI` workflow 在目标 commit 上通过，至少包含 `Backend`、`Frontend`、`Snapless Connected App` 和 `Fusion Benchmark` jobs。
2. 本地或 CI 至少覆盖：
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

优先回滚到上一个已验证镜像 digest，而不是仅依赖浮动 tag：

```bash
docker pull ghcr.io/normojs/data-proxy@sha256:<previous-digest>
```

回滚后需要验证：

- 服务启动成功。
- 数据库迁移没有造成不可逆阻塞；如果有迁移风险，发布前必须准备回滚 SQL 或只读降级方案。
- 企业治理开关可关闭外部 email/webhook 投递。
- V1.3 站内通知仍可读取，审批/审计 deep link 不报错。

## 合规注意事项

- Data Proxy 基于 new-api，必须继续遵守 AGPLv3 和 `NOTICE` 中的 Section 7 附加要求。
- Docker 镜像必须保留 `/licenses/LICENSE`、`/licenses/NOTICE` 和 `/licenses/THIRD-PARTY-LICENSES.md`。
- 修改品牌、页脚、关于页或法律页时，不能移除原项目链接和 attribution 文案。
