# Data Proxy

[![CI](https://github.com/normojs/data-proxy/actions/workflows/ci.yml/badge.svg)](https://github.com/normojs/data-proxy/actions/workflows/ci.yml)
[![Docker](https://github.com/normojs/data-proxy/actions/workflows/data-proxy-docker.yml/badge.svg)](https://github.com/normojs/data-proxy/actions/workflows/data-proxy-docker.yml)
[![License: AGPLv3](https://img.shields.io/badge/license-AGPLv3-brightgreen.svg)](./LICENSE)

Data Proxy 是一个面向企业治理场景的 AI API 网关和额度管控平台，基于开源项目 [new-api](https://github.com/QuantumNous/new-api) 开发。

它继承 new-api 的多模型接入、OpenAI 兼容协议、渠道路由、用户与令牌管理、额度和用量统计能力，并在此基础上增加企业组织、策略额度、审批通知、审计可见性、SSO 同步和发布合规链路。

> [!IMPORTANT]
> 本项目是基于 new-api 的二次开发版本。请保留 [LICENSE](./LICENSE)、[NOTICE](./NOTICE)、[THIRD-PARTY-LICENSES.md](./THIRD-PARTY-LICENSES.md)、原项目链接和 NOTICE 中要求的可见 attribution。Data Proxy 继续遵循 AGPLv3 及 NOTICE Section 7 的附加要求。

## 项目定位

Data Proxy 适合需要集中管理大模型 API 资产的团队：

- 在同一入口管理 OpenAI 兼容、Claude、Gemini、Responses、Realtime、Rerank 等模型协议和渠道。
- 为企业、部门、策略分组和用户设置请求数或 quota 策略。
- 通过 dry-run、hard limit、用量归因和审计日志逐步上线企业治理规则。
- 让临时额度申请、审批结果、过期提醒和外部通知形成可追踪闭环。
- 用 GitHub CI、Docker 发布证据和许可文件分发要求保障发布可回溯。

## 当前能力

### 继承自 new-api 的能力

- OpenAI 兼容 API 网关，支持多模型、多渠道和自动重试。
- 用户、令牌、分组、模型权限、额度、计费和统计仪表盘。
- 多种登录和 OAuth/OIDC 接入能力。
- 与 One API 数据结构的兼容迁移基础。
- Docker、Compose、环境变量和初始化向导部署路径。

更多上游能力可以参考 [new-api 官方文档](https://docs.newapi.pro/) 和 [new-api 仓库](https://github.com/QuantumNous/new-api)。

### Data Proxy 增强能力

- 企业治理模型：企业、组织部门、成员、策略分组、额度策略、用量归因和审计日志。
- 额度策略：支持 `request_count` 和 `quota`，可按企业、部门、分组或用户命中，支持 dry-run 与 hard reject。
- 临时额度审批：用户提交、管理员审批、拒绝、撤回、过期和即将过期提醒。
- 通知闭环：站内通知、企业审计事件、email/webhook outbox、通知偏好、投递结果查询、失败重试和 worker 指标。
- HStation OAuth：登录、注册、绑定、解绑、管理员配置和自动化测试覆盖。
- SSO 组织同步：支持 payload preview、dry-run、冲突列表、事务 apply 和同步审计。
- 企业额度 Redis 计数：可选 Redis 原子 reserve/settle/refund，DB 降级和 DB/Redis 对账修复。
- 高级治理动作：支持模型降级、企业排队、共享池、异常保护和队列 replay；排队请求可记录审计生命周期，并支持 inline JSON、大 payload DB 或本地/S3 对象存储持久化、payload TTL 清理和 multipart/audio upload 重放。
- fusion-benchmark：离线数据集、配置校验、fixture、自检和 CI 检查脚本。

## 快速开始

### 使用 Docker Compose

```bash
git clone https://github.com/normojs/data-proxy.git
cd data-proxy
docker compose up -d data-proxy
```

启动后访问：

```text
http://localhost:3000
```

首次安装请优先使用初始化向导配置数据库和 Redis，然后创建第一个管理员账号。显式环境变量仍然支持，但更适合高级运维覆盖。

### 使用本地依赖

如果希望 Compose 同时启动 PostgreSQL 和 Redis：

```bash
docker compose --profile local-deps up -d
```

初始化向导中使用：

- PostgreSQL host: `postgres`
- Redis host: `redis`

这些本地依赖默认只在 Compose 网络内可见，不会占用宿主机的 `5432` 或 `6379` 端口。

### 使用已发布镜像

```bash
docker pull ghcr.io/normojs/data-proxy:latest
```

稳定版本示例：

```bash
docker pull ghcr.io/normojs/data-proxy:v1.3.0
```

发布、tag、镜像摘要和回滚流程见 [Data Proxy Release Runbook](./docs/data-proxy-release-runbook.md)。

## 常用配置

首次安装推荐通过 Web 初始化向导写入 runtime config。高级场景可以使用 `.env.example` 中的环境变量覆盖。

常见变量：

| 变量 | 说明 |
| --- | --- |
| `SQL_DSN` | 数据库连接字符串，高级覆盖项。 |
| `REDIS_CONN_STRING` | Redis 连接字符串，高级覆盖项。 |
| `SESSION_SECRET` | 多节点部署必须设置的会话密钥。 |
| `NODE_TYPE` | 主节点可设为 `master`，用于周期任务。 |
| `NODE_NAME` | 节点名称，会进入审计和运维排查链路。 |
| `DATA_PROXY_SETUP_AUTO_RESTART` | 控制初始化向导保存配置后是否自动触发容器重启。 |
| `ENTERPRISE_QUEUE_PAYLOAD_TTL_SECONDS` | queue replay payload 保留秒数，默认 7 天；只清理已 released 的旧 payload 和旧孤儿 payload。 |
| `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_PROVIDER` | queue replay 大 payload 外部对象存储 provider；未设置时使用 DB，支持 `local` 或 `s3`。 |
| `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_DIR` | `local` provider 的对象目录；未设置时使用系统临时目录。 |
| `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_ENDPOINT` / `ENTERPRISE_QUEUE_PAYLOAD_OBJECT_S3_BUCKET` | `s3` provider 的 S3 或 S3-compatible endpoint 和 bucket。 |

完整部署说明见 [Data Proxy Operator Guide](./docs/data-proxy-operator-guide.md)。

## 管理与验证

### 企业治理入口

- 管理端入口：`Admin` -> `Enterprise Governance`
- 路由：`/enterprise`
- 权限：管理员及以上

建议上线顺序：

1. 保持企业治理关闭，确认现有网关和计费链路不受影响。
2. 开启 dry-run，观察策略命中、would reject 审计和用量归因。
3. 对测试用户或测试分组开启小范围 hard limit。
4. 再扩大到真实部门或企业级策略。

详细操作见 [Enterprise Governance Admin Guide](./docs/enterprise-governance-admin-guide.md)。

### 本地验证

常用验证命令：

```bash
git diff --check
go test ./model ./controller ./service ./router ./oauth
cd web/default && bun run typecheck
cd web/default && bun run smoke:approval-notification-links
cd web/default && NODE_OPTIONS=--max-old-space-size=4096 bun run build
scripts/fusion-benchmark-check.sh
```

发布前建议运行完整预检：

```bash
make deployment-preflight
```

可选 Docker 构建预检：

```bash
DEPLOYMENT_PREFLIGHT_DOCKER_BUILD=1 make deployment-preflight
```

## 文档索引

| 文档 | 用途 |
| --- | --- |
| [Data Proxy Operator Guide](./docs/data-proxy-operator-guide.md) | 运行、初始化、依赖和部署交接。 |
| [Data Proxy Release Runbook](./docs/data-proxy-release-runbook.md) | tag、镜像、发布证据、回滚和合规检查。 |
| [Deployment Readiness](./docs/deployment-readiness.md) | 发布前预检命令和当前机器状态记录。 |
| [Enterprise Governance Admin Guide](./docs/enterprise-governance-admin-guide.md) | 企业治理管理员操作手册。 |
| [Post V1.3 TODO](./docs/data-proxy-post-v1.3-todo.md) | V1.3 之后的开发顺序和剩余任务。 |
| [Branding and Release Policy](./docs/branding-and-release-policy.md) | Data Proxy 品牌边界和 new-api attribution 规则。 |

仓库中的 `README.en.md`、`README.zh_CN.md`、`README.zh_TW.md`、`README.fr.md`、`README.ja.md` 仍保留为上游 new-api 资料和历史 attribution 参考。Data Proxy 的运行和发布入口以本 README 及 `docs/data-proxy-*` 文档为准。

## 开源协议与合规

Data Proxy 基于 [new-api](https://github.com/QuantumNous/new-api) 开发，继续采用 [GNU Affero General Public License v3.0](./LICENSE)。

请注意：

- 分发源码、镜像、二进制、前端 bundle 或桌面安装包时，必须保留 `LICENSE`、`NOTICE` 和 `THIRD-PARTY-LICENSES.md`。
- 修改版不能误导软件来源，需要清楚标记 Data Proxy 的变更来源。
- 带 UI 的修改版必须保留 NOTICE 中要求的可见 attribution 文案和原项目链接。
- Docker 镜像发布链路应继续携带 `/licenses/LICENSE`、`/licenses/NOTICE` 和 `/licenses/THIRD-PARTY-LICENSES.md`。

如果你的组织不能接受 AGPLv3 或 NOTICE Section 7 的义务，请在部署、分发或提供网络服务前先完成内部法务评估。

## 合法使用

本项目仅适用于合法、授权的 AI API 网关、企业组织认证、多模型管理、用量分析、成本核算和私有化部署场景。

使用者需要自行合法取得上游 API key、账号、模型服务和接口授权，并遵守上游服务条款及适用法律法规。若向公众提供生成式 AI 服务或 API 转售服务，应先完成所在地要求的备案、许可、内容安全、实名、日志留存、税务、支付和上游授权义务。
