# 一键部署 Data Proxy

面向首次部署的运维/开发者：用最少步骤把服务跑起来并完成一次可用请求。

## 路径 A：本机最快（开发 / 试用）

```bash
git clone <your-repo-url> data-proxy
cd data-proxy/upstream/new-api   # 或你的代码根目录
docker compose up -d --build
```

1. 打开 `http://localhost:3000`
2. 完成初始化向导（数据库 / 可选 Redis）
3. 创建首个管理员
4. 添加至少一个上游渠道与模型
5. 创建用户 API Key，按 [user-quickstart.md](./user-quickstart.md) 发一次请求
6. 健康检查：`curl -s http://127.0.0.1:3000/api/status`

可选本地依赖（不占用宿主机端口）：

```bash
docker compose --profile local-deps up -d
```

向导里数据库主机填 `postgres`，Redis 主机填 `redis`。

## 路径 B：生产最小 compose

文件：

- `docker-compose.prod.yml`（主服务）
- `docker-compose.wechat-pay.yml`（可选；无微信支付可准备空 `secrets/wechatpay` 目录）

```bash
cp .env.example.minimal .env.production
# 编辑 .env.production：SESSION_SECRET、SQL_DSN、FRONTEND_BASE_URL 等

mkdir -p secrets/wechatpay data logs
# 拉取/导入镜像后：
./scripts/prod-compose.sh up -d
# 或：
# docker compose -f docker-compose.prod.yml -f docker-compose.wechat-pay.yml --env-file .env.production up -d
```

健康检查（默认绑定 `127.0.0.1:13002`）：

```bash
curl -s http://127.0.0.1:13002/api/status
```

更完整的脚本：`scripts/prod-deploy.sh`、`scripts/prod-rollback.sh`。

## 环境变量（必填一屏）

见 [`.env.example.minimal`](../.env.example.minimal)。完整高级项见 [`.env.example`](../.env.example)。

| 变量 | 说明 |
|------|------|
| `SESSION_SECRET` | 多机/生产必改随机串 |
| `SQL_DSN` | MySQL 或 PostgreSQL（也可用向导配置） |
| `FRONTEND_BASE_URL` | 公网访问根 URL |
| `REDIS_CONN_STRING` | 可选，推荐生产开启 |
| `TZ` | 时区，默认 `Asia/Shanghai` |

## 验收清单

- [ ] `/api/status` 返回 `success: true`
- [ ] 管理员可登录控制台
- [ ] 至少 1 个渠道 + 模型可用
- [ ] 用户 Key 可调用 `/v1/chat/completions`
- [ ] （可选）Monitoring 应用「安全故障切换预设」

## 相关文档

- 终端用户接入：[user-quickstart.md](./user-quickstart.md)
- 渠道故障切换：[channel-failover-and-circuit-breaker.md](./channel-failover-and-circuit-breaker.md)
- 运维手册：[data-proxy-operator-guide.md](./data-proxy-operator-guide.md)
