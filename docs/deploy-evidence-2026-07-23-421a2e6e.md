# 生产部署证据 2026-07-23：`sha-421a2e6e`

## 结论

**PASS**：`https://dp.app.mbu.ltd` 已从 `sha-fdf8fd73` 升级到 **`sha-421a2e6e`**（`/connect/device` 展示 Token group）。

## 环境

| 项 | 值 |
| --- | --- |
| 主机 | `47.122.29.88`（snsc-prod-应用2） |
| 提交 | `421a2e6e` |
| Package CI | run `29972015325` success |
| 包 | `data-proxy-421a2e6e-linux-amd64.tar.gz` |
| SHA256 | `617d7115a1455dbfb89a9b5a48ce8c2771efd90c9738065175f9580458dbed99` |
| 镜像 | `data-proxy:421a2e6e` |
| 回滚归档 | `image-archive/20260723T013643Z_data-proxy_fdf8fd73.tar` |

## 步骤

1. Package 产物下载 → Electerm SFTP 上传  
2. `sha256sum -c` OK；归档 `data-proxy:fdf8fd73`  
3. pin compose image → `data-proxy:421a2e6e`；force-recreate  
4. 本机 health / version = `sha-421a2e6e`

## 验收

| 检查 | 结果 |
| --- | --- |
| 容器 | `data-proxy:421a2e6e` healthy |
| 公网 version | `sha-421a2e6e` |
| production smoke（无 Key） | 公开面 passed |
| 功能 | device status `app.default_token_group` + 授权页「Token group」行 |

## 本版上线能力

- Device status / authorize 响应中 `app.default_token_group`
- Token summary 含 `group`（已签发时）
- `/connect/device` 信息区展示令牌分组（优先已签发 token.group，否则 app 默认分组）
