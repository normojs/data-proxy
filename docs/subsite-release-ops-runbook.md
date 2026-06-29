# 子站发布与运维 Runbook

本文档用于子站功能上线前验收和上线后排障。更大的版本发布流程仍以
`docs/data-proxy-release-runbook.md` 和 `docs/deployment-readiness.md` 为准。

## 功能入口

用户侧页面：

- `/s/:slug`：子站入口，按公开状态进入登录、注册或状态页。
- `/s/:slug/login`：子站登录，登录后回到当前子站控制台。
- `/s/:slug/register`：子站注册，受注册策略、邀请码和邮箱域名白名单控制。
- `/s/:slug/dashboard`：无主站侧栏的子站控制台，展示 Base URL、Key、额度、统计和最近调用。
- `/s/:slug/v1`：OpenAI 兼容 API Base URL，只接受绑定当前子站的 Key。

用户侧 API：

- `GET /api/subsites/:slug/public`
- `POST /api/subsites/:slug/register`
- `GET /api/subsites/:slug/member/self`
- `GET /api/subsites/:slug/dashboard`
- `POST /api/subsites/:slug/token`
- `POST /api/subsites/:slug/token/key`
- `POST /api/subsites/:slug/token/rotate`

管理侧入口：

- UI：`/dashboard/subsites`
- API：`/api/subsite-management/subsites...`

配置路径都在 `/dashboard/subsites`：基础信息、状态和有效期、公告、注册策略、额度策略、成员、渠道、开放模型和模型展示名称。子站渠道上游 Key 加密保存，编辑页和接口都不回显完整 Key。

## 状态和错误码

子站公开状态由 `GET /api/subsites/:slug/public` 暴露，relay 侧由
`middleware.SubsiteContext(true)` 做状态 gate。

- `enabled`：允许进入页面和 `/s/:slug/v1`。
- `disabled`：页面显示关闭页，API 返回 `subsite_disabled`。
- `draft`：仅管理/预览语义，不对普通访问开放。
- `not_started`：页面显示未开始，API 返回 `subsite_not_started`。
- `expired`：由有效期计算，页面显示活动结束，API 返回 `subsite_expired`。
- slug 不存在：页面显示不存在，API 返回 `subsite_not_found`。

上线前至少跑子站状态 smoke：

```bash
cd web/default
node scripts/check-subsite-state-smoke.mjs
```

该脚本覆盖桌面和移动端的关闭页、过期页、不存在页、额度超限页，并检查页面没有水平溢出。
如果本地 shell 没有 Node，把项目可用的 Node runtime 加到 `PATH` 后再运行同一条命令。

## Redis 原子额度计数

子站额度有站点级和用户级两类，窗口有每日窗口和滚动窗口两类。请求前调用
`PreCheckSubsiteQuota`，按最严格规则预留估算额度和 1 次请求；请求完成后调用
`SettleSubsiteQuotaUsage`，按实际 quota 写入计数器。

Redis 可用时，额度预留、结算和回滚使用 Lua 脚本保证单 key 原子性。Redis key 格式：

```text
subsite_quota_counter:v1:<subsite_id>:<user_id>:<scope>:<window_type>:<window_start>
```

字段包括 `used_quota`、`request_count`、`reserved_quota`、`reserved_requests`、
`quota_limit`、`request_limit`。预留时会用 DB 当前值 seed Redis；Redis 报错或不可用时，
系统写日志并降级到 DB 事务路径。

超限错误码：

- `subsite_quota_exceeded`：站点级 quota 超限。
- `subsite_user_quota_exceeded`：用户级 quota 超限。
- `subsite_rate_limited`：站点或用户请求次数超限。

## 流式失败策略

子站额度跟随现有账单链路。请求前 Redis 只做预留；真正是否结算由已有 billing 路径决定：

- 有实际消耗并进入结算时，`SettleBilling` 调用 `SettleSubsiteQuotaUsage`，DB 写入真实 quota，Redis 将 `reserved_*` 转成真实 `used_quota` 和 `request_count`。
- 请求失败且账单路径退款/未结算时，调用 `RefundSubsiteQuotaReservation` 回滚 Redis 预留。
- DB 结算失败时，也会先回滚 Redis 预留，避免 Redis 侧长期占用额度。

排障时先确认对应请求是否产生 settled 的 `model_request` debit 账本事件，再看子站计数器是否有相同 `subsite_id`、窗口和用户维度记录。

## 最近成交价

模型广场和模型详情的实际成交价来自 settled billing events：

- 数据源：`source=model_request`、`event_type=debit`、`status=settled`。
- 默认窗口：最近 1 小时。
- 优先展示最近 1 小时内目标模型/分组的加权实际价。
- 如果最近 1 小时没有目标模型/分组成交，但历史上有成交，则展示最近一笔历史 settled 成交价，并返回 `is_fallback=true`、`price_may_have_changed=true`、`last_transaction_at`。
- 如果最近 1 小时没有成交但昨天有，会显示昨天那笔或更近的上次成交价，并在前端标注价格可能已变化。
- 如果目标模型/分组没有任何历史 settled 成交，则不显示实际成交价。

“按分组定价”中的分组倍率应保留后台配置值，`0` 是有效配置，不应被前端或后端显示成 `1`。

## 日志、渠道和额度排障

日志：

- 通用 usage logs 支持 `subsite_id` 过滤。
- 子站用户控制台只展示当前用户在当前子站的日志。
- 子站管理页展示当前子站全部日志和 24 小时统计。
- request trace 和 diagnostic candidates 支持子站维度，排障时优先按 request id 和 `subsite_id` 交叉确认。

渠道：

- 子站渠道必须带 `subsite_id`，只参与当前子站 `/s/:slug/v1` 路由。
- 管理侧只展示当前子站渠道；主站管理员可以兜底查看和接管。
- 渠道 Key 不应完整回显；需要确认密钥时使用受保护的取 key 接口或重新保存。

额度：

- 先看 `subsite_quota_policies` 是否配置了站点/用户日额度、滚动窗口额度或请求次数。
- 再看 `subsite_quota_counters` 中相同 `subsite_id`、`user_id`、`scope`、`window_type`、`window_start` 的记录。
- Redis 开启时，检查同窗口 key 的 `reserved_*` 是否长期不归零；如果有，重点查请求失败路径是否调用了退款。
- 页面显示的恢复时间来自当前窗口结束时间；如果窗口配置刚变更，先确认管理页保存后的策略已生效。

## 发布前验证

本次子站上线候选建议记录以下命令输出：

```bash
git diff --check
go test ./... -count=1
cd web/default && ./node_modules/.bin/tsc -b
cd web/default && ./node_modules/.bin/rsbuild build
cd web/default && node scripts/check-subsite-state-smoke.mjs
docker compose config
docker compose -f docker-compose.dev.yml config
docker compose -f docker-compose.migration.yml config
scripts/prod-compose.sh config
scripts/data-proxy-release-gate.sh --with-docker-config
make mcp-migration-docker
docker build --target builder2 -t data-proxy:preflight-builder .
docker build -t data-proxy:preflight-runtime .
```

完整镜像构建后，还要启动一次临时容器并检查 `/api/status` 返回 `success=true`。发布证据至少记录 commit、镜像 tag/digest、迁移验证结果、Docker compose config 结果、状态页 smoke 结果和回滚镜像。
