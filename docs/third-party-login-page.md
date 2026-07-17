# 第三方登录专页

## 入口

- 专页：`/oauth-login`（支持 `?redirect=/dashboard`）
- 密码登录页顶部链到专页
- 专页底部可回密码登录 `/sign-in`

## 已支持提供商（由 `/api/status` 开关控制）

| 提供商 | status 字段 | 备注 |
| --- | --- | --- |
| GitHub | `github_oauth` | `/api/oauth/github` |
| Discord | `discord_oauth` | |
| OIDC | `oidc_enabled` | 通用 OIDC |
| LinuxDO | `linuxdo_oauth` | |
| H 站 | `hstation_oauth` | |
| WeChat | `wechat_login` | 扫码 + 验证码弹窗 |
| Telegram | `telegram_oauth` | Login Widget |
| 自定义 OAuth | `custom_oauth_providers[]` | 管理端可配 |

回调页仍为：

- `/oauth/:provider`（标准 OAuth code 回调）
- `/(auth)/oauth`（兼容 wechat 等）

## 管理员配置

系统设置 → Auth / OAuth：开启对应提供商并填写 Client ID/Secret、回调域名。

回调域名需包含站点公网地址，例如：

```text
https://dp.app.mbu.ltd/oauth/github
https://dp.app.mbu.ltd/oauth/discord
https://dp.app.mbu.ltd/oauth/<custom-slug>
```

## 产品目标

为外链/分享提供「只展示第三方登录」的干净页面，避免密码表单干扰。
