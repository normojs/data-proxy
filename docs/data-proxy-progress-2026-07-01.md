# Data Proxy 进度文档

日期：2026-07-01  
代码目录：`/Users/fushilu/workspace/revocloud/data-proxy/upstream/new-api`

## 当前结论

本轮开发代码已经完成并提交到本地主分支，当前生产补丁提交为：

```text
c5738baf fix: record stream status in error logs
```

已经完成的主线包括：模型展示名称、usage logs 默认分页调整、模型筛选下拉宽度、分组价格倍率和最近成交价 fallback、Redis 重新初始化兼容、流式失败细分、流式 chunk 内容错误映射。

生产部署、生产配置落地和真实流式映射冒烟已经完成。当前生产镜像已切到 `data-proxy:c5738baf`，容器健康检查通过；线上渠道 `GTP免费2` 已写入 `settings.stream_error_mapping`，真实流式请求已命中 `upstream_key_sleeping` 映射，并已确认错误日志包含 `stream_status`。

继续排查时发现一个后续补丁点：错误日志路径原本没有把首包前流式映射失败的 `stream_status` 写入 `logs.other`。该修复已提交并部署为 `data-proxy:c5738baf`，usage logs 现在可直接看到 `mapped_error` 元数据。

## 已完成提交

| 提交       | 内容                                                                                                                   | 状态         |
| ---------- | ---------------------------------------------------------------------------------------------------------------------- | ------------ |
| `80eb8b4b` | 修复模型筛选下拉框过窄；修复模型广场和详情页的分组价格倍率；最近一小时无成交价时回退显示上次成交价，并提示价格可能变化 | 已完成       |
| `944a6ff4` | usage logs 默认分页改为桌面 20、移动 10                                                                                | 已完成       |
| `e9ff7c57` | 支持模型展示名称；模型列表、模型广场、模型详情和定价相关视图使用展示名称                                               | 已完成       |
| `48affd81` | 增加流式失败细分，usage logs 展示流式结束原因、失败类别和映射错误信息                                                  | 已完成       |
| `20776d43` | 支持 Redis 重新初始化；修复 PostgreSQL 下实际成交价查询                                                                | 已完成       |
| `b4e6b77d` | 修复已有 JSON 状态码映射在可视编辑器里显示为未配置的问题                                                               | 已完成       |
| `5ceae0d0` | 支持从流式 chunk 内容映射为错误码、错误信息和可重试错误                                                                | 已完成       |
| `c5738baf` | 修复错误日志缺少流式 `stream_status`；补充生产进度文档                                                                 | 已完成并部署 |

## 流式 Chunk 错误映射

新增渠道设置字段：`settings.stream_error_mapping`。

支持能力：

- `target`：`raw`、`text` 或默认全部。
- `operator`：`contains`、`equals` / `eq`、`prefix` / `starts_with`、`suffix` / `ends_with`、`regex`。
- 支持 HTTP 200 的 SSE `data:` chunk 内容识别。
- `target=text` 会递归提取 JSON 字符串；非 JSON chunk 会回退按原始文本匹配。
- 匹配发生在任何下游写入前：返回 `NewAPIError`，走现有重试和故障切换。
- 匹配发生在下游已经写入后：记录流式失败元数据并停止流，但不再对客户端伪造上游重试。

建议给线上渠道 `GTP免费2` 配置：

```json
[
  {
    "enabled": true,
    "name": "公益 token 睡眠",
    "target": "text",
    "operator": "contains",
    "pattern": "公益token睡眠中",
    "status_code": 429,
    "error_code": "upstream_key_sleeping",
    "message": "上游公益 token 睡眠中，请稍后重试或切换 key",
    "retryable": true,
    "channel_failure_candidate": true,
    "max_chunks": 3
  }
]
```

该配置可覆盖线上返回内容：

```text
公益token睡眠中 https://dc.hhhl.cc/chat/room/amlc1bekzi
```

预期 usage logs 里的流式失败元数据：

```text
end_reason: mapped_error
failure_category: upstream_mapped_error
mapped_error_code: upstream_key_sleeping
mapped_status_code: 429
mapped_rule: 公益 token 睡眠
```

## 本地验证

已通过的验证：

```bash
go test ./relay/helper ./relay/common ./service
go test ./relay ./relay/channel/openai ./relay/channel/claude ./relay/channel/gemini ./relay/channel/baidu ./relay/channel/dify ./relay/channel/xai
go test ./...
cd web/default && ./node_modules/.bin/tsc -b
cd web/default && ./node_modules/.bin/rsbuild build
```

本地 Docker 镜像构建结果（流式映射主版本）：

```text
image: data-proxy:5ceae0d0
image id: sha256:94160acd5ec0d9703ff9a1f2091f68e7ee3ba89bac4b4d7049b0890072f65b37
```

后续错误日志补丁镜像：

```text
image: data-proxy:c5738baf
image id: sha256:4ab6f42f3b48b7ed37d7792c6e6e691629eb2dbfc52d95387d0be570070522a0
package sha256: 7fa22e6583fa2bb33a90fefb00dd5bbceea89d7abfed1c7ebbbf06132b242b36
```

本地部署包：

```text
/tmp/data-proxy-deploy-5ceae0d0/data-proxy-5ceae0d0-local-linux-amd64.tar.gz
/tmp/data-proxy-deploy-5ceae0d0/data-proxy-5ceae0d0-local-linux-amd64.sha256
/tmp/data-proxy-deploy-5ceae0d0/data-proxy-remote-deploy-5ceae0d0.sh

/tmp/data-proxy-deploy-c5738baf/data-proxy-c5738baf-local-linux-amd64.tar.gz
/tmp/data-proxy-deploy-c5738baf/data-proxy-c5738baf-local-linux-amd64.sha256
/tmp/data-proxy-deploy-c5738baf/data-proxy-remote-deploy-c5738baf.sh
```

包校验：

```text
b79f450b1956bdfad09bc97618623fefcc6cf49eba3efca400745203e60bafd5  data-proxy-5ceae0d0-local-linux-amd64.tar.gz
7fa22e6583fa2bb33a90fefb00dd5bbceea89d7abfed1c7ebbbf06132b242b36  data-proxy-c5738baf-local-linux-amd64.tar.gz
```

## 当前部署状态

部署状态：已完成。

远程生产信息：

```text
应用目录: /root/workspace/dataproxy/data-proxy
镜像归档目录: /root/workspace/dataproxy/image-archive
Compose 文件: docker-compose.prod.yml + docker-compose.wechat-pay.yml
服务/容器: data-proxy
本机健康检查: http://127.0.0.1:13002/api/status
公网健康检查: https://dp.app.mbu.ltd/api/status
```

当前连接状态：

- Electerm MCP 端口 `127.0.0.1:30837` 可用，当前 SSH tab 为 `PRmZrQc_2026-07-01-19-29-38`。
- 直接 SSH `root@dp.app.mbu.ltd` 仍然被拒绝：`Permission denied`。
- 本次生产部署通过 Electerm MCP/SFTP 完成，后续仍优先使用 Electerm 连接生产。

## 已执行部署步骤

1. 已通过 Electerm MCP/SFTP 上传以下文件到 `/root/workspace/dataproxy/data-proxy/`：

```text
data-proxy-5ceae0d0-local-linux-amd64.tar.gz
data-proxy-5ceae0d0-local-linux-amd64.sha256
data-proxy-remote-deploy-5ceae0d0.sh
```

2. 已完成远程 checksum 校验：

```text
b79f450b1956bdfad09bc97618623fefcc6cf49eba3efca400745203e60bafd5
```

3. 已执行远程部署脚本，当前生产 compose 镜像为：

```text
data-proxy:5ceae0d0
```

后续已部署错误日志补丁镜像：

```text
data-proxy:c5738baf
```

4. 已完成部署后验证：

```text
container: data-proxy Up (healthy)
local health: http://127.0.0.1:13002/api/status OK
public health: https://dp.app.mbu.ltd/api/status OK
current version: c5738baf
```

5. 已归档上一版镜像用于回滚：

```text
/root/workspace/dataproxy/image-archive/20260701T113433Z_data-proxy_b4e6b77d.tar
/root/workspace/dataproxy/image-archive/20260701T123431Z_data-proxy_5ceae0d0.tar
```

## 生产配置状态

已给线上渠道 `GTP免费2`（channel id `12`）写入 `settings.stream_error_mapping`。

配置写入前已备份原始 settings：

```text
/root/workspace/dataproxy/data-proxy/backups/channel-settings/20260701T115233Z_channel_12_settings.json
```

配置写入后已重启 `data-proxy` 服务使配置重新加载，容器仍为 healthy。

注意：

- 不要把 `error_code` 配成 `channel:*` 这类带通道语义的模板值。
- 推荐使用稳定错误码：`upstream_key_sleeping`。
- 不要在文档、终端输出或聊天里打印渠道密钥、`.env.production`、支付证书或生产数据库密码。

## 生产冒烟清单

全链路冒烟：

- [ ] 模型广场：展示名称、模型筛选下拉宽度、分组价格倍率、最近成交价 fallback。
- [ ] 模型详情：展示名称、按分组定价倍率、最近一小时成交价；若一小时无成交价则显示上次成交价并标注可能有价格变化。
- [ ] usage logs：common 分组、默认分页桌面 20/移动 10、流式失败细分字段。
- [ ] 渠道状态码映射：已有 JSON 在可视模式可见。
- [x] 渠道 `GTP免费2`：HTTP 200 流式文本 `公益token睡眠中 ...` 已命中 `upstream_key_sleeping`。规则原始 `mapped_status_code=429`，因渠道 `status_code_mapping` 存在 `{ "429": "503" }`，客户端和 error log 最终 HTTP 状态为 `503`。

Redis/P6 行为验证：

- [ ] 普通请求成功计数正确。
- [ ] 流式成功计数正确。
- [ ] 流式失败计数正确。
- [ ] 中途断流计数和日志分类正确。
- [ ] 映射错误命中时，失败类别为 `upstream_mapped_error`。

部署回滚准备：

- [x] 记录部署前镜像：`data-proxy:b4e6b77d`。
- [x] 确认 `/root/workspace/dataproxy/image-archive` 有上一版镜像归档。
- [x] 记录本次镜像 `data-proxy:5ceae0d0` 和补丁镜像 `data-proxy:c5738baf` 的包 sha256。
- [x] 确认 `docker-compose.prod.yml` 和 `docker-compose.wechat-pay.yml` 可正常启动。

## 当前工作区注意事项

当前主工作区存在与本轮部署无关的未提交 playground 改动：

```text
web/default/src/features/playground/api.ts
web/default/src/features/playground/components/playground-chat.tsx
web/default/src/features/playground/hooks/use-chat-handler.ts
web/default/src/features/playground/hooks/use-stream-request.ts
web/default/src/features/playground/lib/message-utils.ts
web/default/src/features/playground/types.ts
web/default/src/features/playground/components/message-details.tsx
```

这些文件不要混入本次生产部署提交，也不要随手回滚。生产部署包均从干净 worktree 构建，主版本基于 `5ceae0d0`，错误日志补丁基于 `c5738baf`。

## 下一步

优先顺序：

1. 可选继续跑前端 UI 冒烟：模型广场、模型详情、usage logs 分页和渠道可视编辑器。
2. 可选继续验证 Redis/P6 计数：普通成功、流式成功、流式失败、中途断流、映射错误。

## 生产冒烟结果

真实流式映射冒烟已完成：

```text
request_id: 202607011159285773124008268d9d64O4Up22x
error_log_id: 28987
channel_id: 12
token_id: 21
model: gpt-5.5
client_status: 503
error_code: upstream_key_sleeping
message: 上游公益 token 睡眠中，请稍后重试或切换 key
```

Docker 日志确认命中：

```text
stream error mapping matched: rule=公益 token 睡眠 code=upstream_key_sleeping status=429
stream ended: reason=mapped_error
```

状态码差异已确认：渠道 `status_code_mapping` 当前包含 `429 -> 503`，所以流式规则内部记录 `mapped_status_code=429`，经过现有渠道状态码映射后，对客户端和 error log 的最终 `status_code` 为 `503`。

## 后续本地修复

已提交并部署错误日志缺少 `stream_status` 的修复：

- `service.AppendStreamStatus` 从成功日志路径抽出复用。
- `controller.processChannelError` 记录错误日志时追加 `stream_status`。
- 新增测试覆盖：最终 error status 为 `503` 时，`stream_status.mapped_status_code` 仍保留规则原始 `429`。
- 已构建并部署 `data-proxy:c5738baf`，生产健康检查返回 `version=c5738baf`。

已通过验证：

```bash
go test ./service ./controller -run 'TestGenerateTextOtherInfoIncludes|TestProcessChannelErrorRecordsStreamStatusInErrorLog|TestShouldRetryUsesTransientFailureRulesForFailover|TestChannelFailoverTrace'
```

补丁部署后真实流式冒烟再次完成：

```text
request_id: 202607011238366288041108268d9d6ETkdb4dT
error_log_id: 28988
client_status: 503
error_code: upstream_key_sleeping
stream_status.end_reason: mapped_error
stream_status.failure_category: upstream_mapped_error
stream_status.mapped_error_code: upstream_key_sleeping
stream_status.mapped_status_code: 429
stream_status.mapped_rule: 公益 token 睡眠
```

## 后续前端 i18n 整理（已部署）

已在本地继续整理前端翻译和子站文案：

- 补齐 `zh.json` 运行时 literal key 的中文翻译；当前扫描结果为 `missingZh=0`。
- 修复 subsite 里 `Open` 的语义混用：状态改为 `Available`，无限配额改为 `Unlimited`，注册策略改为 `Open registration`。
- 将渠道抽屉 Responses 推理适配里直接使用中文作为 `t()` key 的文案改为英文 key，并补充对应中文翻译，避免英文/其他语言界面泄露中文 fallback。
- 修复 `sync-i18n.mjs` 的基准语言策略：固定以 `en` 为 source locale，并用 locale key union 保留其他语言已有额外 key；同时增加重复翻译 key 报告。
- 将源码扫描纳入 `i18n:sync`：自动收集 `t('...')` 与 `STATIC_I18N_KEYS`，并把缺失的运行时 key 回填到 `en` source locale。
- 清理 `en.json`、`zh.json` 中的重复翻译 key；当前所有 locale 重复 key 扫描结果为 `dupes=0`、`conflicts=0`。

已通过验证：

- `en` / `zh` 运行时 key 缺失均为 `0`。
- `fr` / `ja` / `ru` / `vi` 新增 key 已完成真实翻译，不再是英文 fallback。
- `i18n:sync` 报告里 `sourceMissingRuntimeKeyCount=0`，所有 locale `missingCount=0`、`untranslatedCount=0`、`duplicateKeyCount=0`。
- 修复本地 Bun 依赖布局中断开的 `@base-ui/react` symlink 后，`tsc -b` 通过。
- `prettier --check`、`git diff --check` 通过。

生产部署记录：

- 构建镜像：`data-proxy:b8e01557-i18n-202607020642`。
- 部署包：`data-proxy-b8e01557-i18n-202607020642-local-linux-amd64.tar.gz`。
- 部署包 sha256：`d2c9412e9fdc3f2f4e46a73adc5ddd549b9e0eeb762f56d4cf357b1e99a832dc`。
- 远端部署目录：`/root/workspace/dataproxy/data-proxy`。
- 回滚镜像归档：`/root/workspace/dataproxy/image-archive/20260701T225334Z_data-proxy_stream-map-drain-583ad4c6-20260701234317.tar`。
- compose 备份：`/root/workspace/dataproxy/data-proxy/docker-compose.prod.yml.bak.20260701T225341Z`。

线上冒烟结果：

- Docker 容器：`data-proxy data-proxy:b8e01557-i18n-202607020642 Up ... (healthy)`。
- 远端 `http://127.0.0.1:13002/api/status`：`success=true`。
- 公网 `https://dp.app.mbu.ltd/api/status`：HTTP 200，`success=true`。
- 公网 `https://dp.app.mbu.ltd/`：HTTP 200。
- 公网 `https://dp.app.mbu.ltd/playground`：HTTP 200。
- 注意：`/api/status` 的 `version` 仍显示 `sha-583ad4c6`，原因是本地 `VERSION` 文件已有未提交变更并参与镜像构建；本次提交不包含该文件。
