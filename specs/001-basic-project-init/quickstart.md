# 快速开始：基础工程初始化

本文档用于验证“基础工程初始化”功能是否按计划完成。命令以仓库根目录为起点。

## 前置条件

- 已安装 Go 1.22 或更高版本。
- 已安装 Node.js 20 LTS 或 22 LTS。
- 已拥有可访问的远程 MySQL 8 和 Redis 7 服务。
- 已获得远程 MySQL 和 Redis 的连接凭据。

## 1. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，填写远程 MySQL 和 Redis 配置：

- `MYSQL_HOST`
- `MYSQL_PORT`
- `MYSQL_USER`
- `MYSQL_PASSWORD`
- `MYSQL_DATABASE`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `REDIS_DB`
- `DESKTOP_API_BASE_URL`

确认 `.env` 没有被提交：

```bash
git status --short
```

## 2. 验证后端骨架

```bash
cd backend
go test ./...
go run ./cmd/server
```

预期结果：

- 服务启动成功。
- `main.go` 只承担启动组装职责。
- 未创建用户、文件、策略、权限等业务表。

## 3. 验证健康检查

保持后端服务运行，在另一个终端执行：

```bash
curl http://localhost:8080/health
```

预期结果：

- 响应包含 `app`、`mysql`、`redis` 三类状态。
- 远程依赖可用时整体状态为 `ok`。
- 任一远程依赖不可用时整体状态为 `degraded`，并包含不泄露敏感信息的失败摘要。

## 4. 验证桌面端

```bash
cd desktop
npm install
npm run typecheck
npm run build
npm run dev
```

预期结果：

- 桌面端窗口可以打开。
- 首页展示项目名称和后端连接状态占位。
- `desktop/src/renderer/api` 存在，并包含后续调用后端的封装入口。

## 5. 验证文档和安全边界

```bash
rg -n "MYSQL_PASSWORD|REDIS_PASSWORD|token|secret|password" .env.example README.md backend desktop
```

预期结果：

- 命中项仅为配置键、占位值或安全说明。
- 不存在真实服务器 IP、真实账号、真实密码、访问令牌或真实密钥。

## 常见问题定位

- 后端无法启动：检查 `.env` 是否存在、`APP_PORT` 是否被占用、Go 依赖是否下载成功。
- MySQL 状态异常：检查 `MYSQL_HOST`、`MYSQL_PORT`、账号密码、数据库名、服务器防火墙和白名单。
- Redis 状态异常：检查 `REDIS_ADDR`、`REDIS_PASSWORD`、`REDIS_DB`、服务器防火墙和白名单。
- 桌面端无法启动：检查 Node.js 版本、依赖安装结果和 Vite/Electron 启动日志。
- 桌面端无法连接后端：检查后端是否运行，以及 `DESKTOP_API_BASE_URL` 是否指向正确地址。
