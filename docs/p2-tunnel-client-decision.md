# P2 客户端隧道主路径决策

日期：2026-07-16  
状态：已冻结  
范围：`docs/product-gap-todo.md` P2-1

## 决策

**主路径：`dpa` / `data-proxy-agent`（Go CLI）**

**辅路径：QidianBrowser Bridge client（浏览器/守护进程）**

## 理由

1. `dpa` 已在 data-proxy 主仓库产品化：enroll、doctor、service、update、tunnel route、stdio/streamable MCP、审计与策略默认关闭危险能力。
2. 控制台已有 Tunnel Apps / Connections / Agent Setup 复制命令，面向 `dpa` 的安装与路由文案完整。
3. QidianBrowser 仍以协议文档 + mock/bridge 雏形为主，正式权限 UI 与危险操作确认未完成；不宜阻塞“云端 Agent → 本机服务”闭环。
4. data-proxy 侧 `bridge` / `qidian_browser` transport 与 `mcp_proxy.*` 协议保持兼容，辅路径可在主路径稳定后并行补齐。

## 非目标

- 本阶段不把 QidianBrowser 作为默认安装/演示路径。
- 不开放默认危险写/exec 能力；继续依赖本机 `policy.*` 显式开启。
- 不做多节点分布式 tunnel 限流。

## 验收主路径（端到端）

1. 安装：`scripts/install-data-proxy-agent.sh` 或 Release 资产。
2. 注册：控制台生成 setup token → `dpa enroll --server <base> --setup-token <token>`。
3. 运行：`dpa run` 或 `dpa service install && dpa service start`。
4. 诊断：`dpa doctor --json`、`dpa status --json` 显示在线。
5. 暴露本地服务：
   - MCP：`dpa mcp add ...` 或配置 stdio/streamable HTTP；
   - HTTP：`dpa tunnel route add http <name> --url http://127.0.0.1:<port>`。
6. 云端调用：通过 Tunnel Connection / MCP Proxy 调用成功，留下 `request_id` 与 tunnel/bridge audit。
7. 计费/拒绝：失败可审计；启用结算时 ledger 可追溯。

## Smoke 工具

- Agent 状态：`scripts/data-proxy-agent-status-smoke.sh`
- Bridge 既有：`docs/mcp-bridge-smoke.md` / `make mcp-bridge-smoke`（若环境具备）
- 生产 agent 安装与 manifest：见 `docs/data-proxy-release-runbook.md`

## 与 product-gap 的对应

- “冻结客户端范围” → 本文。
- “端到端安装→调用” → 以上验收主路径；需在可连生产/预发环境时勾选证据。
- QidianBrowser 真实客户端继续按 `QidianBrowser/docs/REMOTE_BRIDGE_CLIENT.md` 独立推进，不阻塞 dpa 主路径。
