# CP-ABE 加密文件共享系统

本项目用于验证密文策略属性基加密 CP-ABE 在细粒度文件共享场景中的工程应用价值。当前阶段是“基础工程初始化”：搭建可本地开发的 Go 后端和 Electron + TypeScript 桌面端，并通过 `.env` 连接已经部署在个人服务器上的 MySQL 8 和 Redis 7。

## 当前范围

本阶段包含：

- Go 后端基础工程
- Gin 服务启动入口
- Gorm 远程 MySQL 连接初始化
- go-redis 远程 Redis 连接初始化
- `GET /health` 健康检查接口
- Electron + TypeScript + Vite 桌面端基础首页
- 桌面端 API 调用封装结构
- `.env.example`、`.gitignore` 和本启动说明

本阶段不包含：

- 用户注册、登录、权限系统
- 具体业务表、数据库迁移或种子数据
- 默认 MySQL / Redis Docker Compose 服务
- Kubernetes、CI/CD、消息队列
- CP-ABE、RSA、AES-GCM、策略解析、Benchmark 或 Audit 业务能力

## 目录结构

```text
backend/
  cmd/server/main.go
  internal/config/
  internal/router/
  internal/handler/
  internal/service/
  internal/repository/
  internal/model/
  internal/middleware/
  internal/pkg/
desktop/
  package.json
  tsconfig.json
  vite.config.ts
  src/main/
  src/preload/
  src/renderer/
  src/renderer/api/
.env.example
.gitignore
README.md
```

## 环境要求

- Go 1.22 或更高版本
- Node.js 20 LTS 或 22 LTS
- 可访问的远程 MySQL 8
- 可访问的远程 Redis 7

## 配置环境变量

复制示例文件：

```bash
cp .env.example .env
```

编辑 `.env`：

```dotenv
APP_ENV=development
APP_PORT=8080

MYSQL_HOST=your-mysql-host
MYSQL_PORT=3306
MYSQL_USER=your-mysql-user
MYSQL_PASSWORD=your-mysql-password
MYSQL_DATABASE=go_cpabe

REDIS_ADDR=your-redis-host:6379
REDIS_PASSWORD=your-redis-password
REDIS_DB=0

DESKTOP_API_BASE_URL=http://localhost:8080
```

`.env` 只保存在本机，不能提交到仓库。`.env.example` 只能保留占位值或说明性示例。

## 启动后端

```bash
cd backend
go mod tidy
go test ./...
go run ./cmd/server
```

默认服务地址：

```text
http://localhost:8080
```

如果修改了 `APP_PORT`，请使用对应端口访问。

## 健康检查

后端启动后，在另一个终端运行：

```bash
curl http://localhost:8080/health
```

正常响应会包含：

- `app`：后端服务状态
- `mysql`：远程 MySQL 连接状态
- `redis`：远程 Redis 连接状态

当 MySQL 或 Redis 不可用时，整体状态会变为 `degraded`，并返回脱敏后的失败摘要。响应不会包含真实密码、令牌或完整连接串。

## 启动桌面端

```bash
cd desktop
npm install
npm run typecheck
npm run build
npm run dev
```

启动后可以看到基础首页，页面展示项目名称和后端连接状态占位。后续业务页面应复用 `desktop/src/renderer/api` 下的统一请求封装。

## 测试与验证

后端：

```bash
cd backend
go test ./...
```

桌面端：

```bash
cd desktop
npm run typecheck
npm run build
```

敏感信息辅助检查：

```bash
rg -n "password|secret|token|真实|服务器" .env.example README.md backend desktop
```

如果命中配置键、占位值或安全说明是正常的；需要人工确认没有真实服务器 IP、账号、密码、令牌或密钥。

## 常见问题

### 后端无法启动

- 检查 `.env` 是否存在。
- 检查 `APP_PORT` 是否为数字。
- 检查端口是否已被其他进程占用。
- 在 `backend/` 运行 `go mod tidy` 确认依赖可下载。

### MySQL 状态异常

- 检查 `MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_DATABASE`。
- 检查个人服务器防火墙、安全组和访问白名单。
- 确认 MySQL 8 服务正在运行，且账号有访问目标数据库的权限。

### Redis 状态异常

- 检查 `REDIS_ADDR`、`REDIS_PASSWORD`、`REDIS_DB`。
- 检查个人服务器防火墙、安全组和访问白名单。
- 如果 Redis 未启用密码，可以让 `REDIS_PASSWORD` 为空。

### 桌面端无法启动

- 检查 Node.js 版本。
- 删除 `desktop/node_modules/` 后重新运行 `npm install`。
- 先运行 `npm run typecheck` 和 `npm run build` 查看具体错误。

### 桌面端无法连接后端

- 确认后端已经启动。
- 确认 `DESKTOP_API_BASE_URL` 指向正确的后端地址。
- 先用浏览器或 `curl` 访问 `/health`，确认后端可访问。

## 安全边界

本项目用于学习、验证和演示 CP-ABE 应用场景。当前基础工程只承诺本地开发和演示能力，不承诺生产环境安全能力。真实 CP-ABE 加解密必须依赖真实 Go 密码学库，后续实现前仍需继续遵守 SpecKit 的 `spec -> plan -> tasks -> implementation` 流程。
