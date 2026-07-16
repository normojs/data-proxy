# 隧道 UI / HTTPS 补强说明（2026-07-17）

## 产品规则

- **公网访问链接**：始终跟随站点地址，生产为 HTTPS（自动，不在表单里选）。
- **本机服务协议**：默认 HTTP，可选 HTTPS；写入 tunnel app `route.local_scheme`。

## 代码变更

### Backend

- `service/tunnel_http.go`
  - route 配置增加 `local_scheme`（兼容 `target_scheme` / `scheme`）
  - `tunnelHTTPTargetURL` 在 host+port 模式下按 local_scheme 拼 `http://` 或 `https://`
  - 完整 `https://...` 写在 `target_path` 仍可用
- `service/tunnel_http_test.go`：HTTPS local scheme 单测

### Frontend

- 创建 Tunnel App：
  - 默认类型改为 **HTTP Tunnel**
  - 增加 **Local Service Protocol**（HTTP 默认 / HTTPS 可选）
  - 字段白话标签 + 公网/本机链接预览
- Tunnel Connections：
  - 复制 endpoint 优先 `status.server_address`，非本机 http 会抬到 https
  - 创建成功文案强调「公网 HTTPS 链接，完整 key 只显示一次」
- 侧栏新增 **Tunnels** 分组：
  - My Devices / My Tunnel Apps / Connections & Links / Sessions
  - Admin MCP 下增加 Tunnel Apps (Approve)
- Sidebar modules 配置（admin + profile）补 `tunnel` section

## 使用提示

1. 侧栏 **Tunnels → My Devices** 先 enroll dpa  
2. **My Tunnel Apps** 创建 HTTP 隧道，本机协议按服务选择  
3. 管理员审批  
4. **Connections & Links** 创建连接，复制 **Public HTTPS Endpoint**

## 未部署

以上为源码改动；生产 `sha-da5af9b2` 尚未包含本轮 UI/backend 变更，需重新打包部署后控制台才生效。
