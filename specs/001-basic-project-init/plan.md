# 实现计划：基础工程初始化

**分支**：`main` | **日期**：2026-07-05 | **规格**：[spec.md](./spec.md)
**输入**：来自 `/specs/001-basic-project-init/spec.md` 的功能规格，以及用户补充的固定技术栈和推荐项目结构

## 摘要

本功能交付一个可本地开发的桌面端全栈基础工程：后端提供 Go 服务入口、集中配置、远程 MySQL/Redis 连接初始化和 `GET /health` 健康检查；桌面端提供 Electron + TypeScript + Vite 初始化工程、基础首页和后端 API 调用封装入口；根目录提供 `.env.example`、`.gitignore` 和中文 README，帮助开发者完成远程依赖配置、启动、测试和排障。

实现采取分阶段推进：先搭建后端最小可运行骨架，再接入远程依赖健康检查，然后搭建桌面端，最后补齐文档、忽略规则和验证命令。不得实现用户注册、登录、权限系统、业务表、默认 MySQL/Redis 容器、Kubernetes、CI/CD 或消息队列。

## 技术上下文

**语言/版本**：Go 1.22+；TypeScript 5.x；Node.js 20 LTS 或 22 LTS。  
**主要依赖**：Gin、Gorm、MySQL Driver、go-redis、godotenv；Electron、Vite、TypeScript。  
**存储**：MySQL 8 部署在个人服务器上，本阶段只初始化连接，不创建业务表；Redis 7 部署在个人服务器上，本阶段只初始化客户端和连通性检查。  
**测试**：后端使用 `go test ./...` 和 `curl GET /health`；桌面端使用 `npm run typecheck`、`npm run build`、`npm run dev`。  
**目标项目**：Electron + TypeScript 桌面端 + Go 后端的 CP-ABE 加密文件共享系统基础工程。  
**性能目标**：本阶段无算法性能目标；健康检查应在本地开发网络正常时快速返回，依赖异常时提供可排查状态。  
**约束**：配置只能来自 `.env`/环境变量；不得硬编码服务器 IP、数据库密码、Redis 密码或真实密钥；Docker Compose 不是默认要求，不生成 MySQL/Redis 容器。  
**规模/范围**：只完成基础工程初始化、依赖连接和启动验证；保留 User、File、Policy、Crypto、Benchmark、Audit 等后续模块边界，不实现业务功能。

## 宪章检查

*关卡：必须在第 0 阶段研究前通过；第 1 阶段设计后必须再次检查。*

- 混合加密：本功能不实现文件内容加密和 DEK 封装；后端结构预留 `internal/pkg` 和后续 Crypto 模块扩展位置，未违反混合加密原则。
- 真实 CP-ABE：本功能不实现 CP-ABE；不得添加模拟 CP-ABE 加解密或伪随机结果。
- 可插拔算法：本功能只初始化工程边界，不实现算法；后续 Crypto 模块可在 `internal/pkg/crypto` 或独立模块中抽象 `CryptoEngine`。
- 公平基准：本功能不输出 RSA/CP-ABE 结论，不混入 AES-GCM 或 DEK 封装耗时。
- 策略解耦：本功能不实现访问树或 LSSS；后续 Policy 模块可独立扩展策略解析和可视化。
- 可解释性：健康检查必须解释 app、mysql、redis 的状态和安全失败摘要，满足开发环境可解释性。
- 模块边界：推荐目录明确保留 config、router、handler、service、repository、model、middleware、pkg，并为后续核心模块扩展留位置。
- 范围纪律：计划明确排除注册、登录、权限、业务表、默认容器、Kubernetes、CI/CD 和消息队列。
- 语言规范：本计划、`research.md`、`data-model.md`、`quickstart.md` 和 `contracts/` 文档均使用简体中文；代码标识符和工程名保留英文。

**初始结论**：通过。没有需要豁免的宪章偏离。

## 项目结构

```text
backend/
  cmd/
    server/
      main.go
  internal/
    config/
    router/
    handler/
    service/
    repository/
    model/
    middleware/
    pkg/
  go.mod
  go.sum
desktop/
  package.json
  tsconfig.json
  vite.config.ts
  index.html
  src/
    main/
    preload/
    renderer/
      api/
/.env.example
/.gitignore
/README.md
specs/001-basic-project-init/
```

## 第 0 阶段：研究

已生成 [research.md](./research.md)，结论如下：

- 后端采用 Gin + Gorm + go-redis + godotenv 的最小工程组合。
- MySQL 和 Redis 默认连接远程服务，`.env.example` 只放占位值和本地开发示例，不写真实凭据。
- 健康检查返回 `app`、`mysql`、`redis` 三类状态，并对错误进行脱敏摘要。
- 桌面端采用 Electron + TypeScript + Vite，不引入复杂 UI 框架。
- Docker Compose 不作为默认产物；如后续需要，只能作为可选本地备用方案单独规划。

## 第 1 阶段：设计

已生成以下设计产物：

- [data-model.md](./data-model.md)：定义本地环境配置、健康检查结果、桌面端基础首页、后端调用封装、项目启动说明。
- [contracts/health-api.md](./contracts/health-api.md)：定义 `GET /health` 响应契约与错误脱敏要求。
- [contracts/env.md](./contracts/env.md)：定义 `.env.example` 必需配置键、说明和敏感信息边界。
- [contracts/desktop-api.md](./contracts/desktop-api.md)：定义桌面端 API 调用封装边界。
- [quickstart.md](./quickstart.md)：定义分阶段启动和验证命令。

### 阶段拆分与验证命令

**阶段 A：后端骨架**

- 创建 `backend/go.mod`、`backend/cmd/server/main.go` 与 `internal/config`、`internal/router`、`internal/handler` 等目录。
- `main.go` 只负责加载配置、初始化依赖、注册路由和启动服务。
- 验证命令：

```bash
cd backend
go test ./...
go run ./cmd/server
```

**阶段 B：远程 MySQL/Redis 连接与健康检查**

- 从 `.env`/环境变量读取 MySQL 8 与 Redis 7 配置。
- 使用 Gorm 初始化 MySQL 连接，使用 go-redis 初始化 Redis 客户端。
- 集中注册 `GET /health`，返回 app、mysql、redis 状态和脱敏失败摘要。
- 验证命令：

```bash
cp .env.example .env
cd backend
go run ./cmd/server
curl http://localhost:${APP_PORT:-8080}/health
go test ./...
```

**阶段 C：桌面端基础工程**

- 创建 Electron + TypeScript + Vite 工程。
- 首页展示项目名称和后端连接状态占位。
- 预留 `desktop/src/renderer/api`，集中管理 API 基础地址与请求封装。
- 验证命令：

```bash
cd desktop
npm install
npm run typecheck
npm run build
npm run dev
```

**阶段 D：根目录工程文档与安全边界**

- 创建 `.env.example`、`.gitignore` 和中文 `README.md`。
- README 覆盖环境变量、后端启动、桌面端启动、测试运行和常见问题排查。
- `.gitignore` 覆盖 `.env`、依赖产物、构建产物、日志和临时文件。
- 验证命令：

```bash
git status --short
rg -n "password|secret|token|真实|服务器" .env.example README.md backend desktop
```

上述敏感词检查只作为人工复核辅助；如果命中示例字段名或说明文本，需要确认没有真实凭据。

## 第 2 阶段：任务规划

由 `/speckit-tasks` 生成 `tasks.md`。任务应按阶段 A 到 D 拆分，避免一次性生成过大的实现，并为每个阶段保留独立验证命令。

## 第 1 阶段后宪章复查

- 混合加密、真实 CP-ABE、RSA 基线、公平基准、策略解耦均未进入实现范围，无偏离。
- 工程结构为后续 Crypto、Policy、Benchmark、Audit 扩展留出边界，无业务代码散落密码算法逻辑的风险。
- 健康检查和 README 强调开发环境诊断，不承诺生产级监控或安全能力。
- 所有生成文档均为简体中文，符合语言规范。

**复查结论**：通过。没有需要记录的复杂度豁免。

## 复杂度跟踪

无。当前计划没有违反宪章的设计取舍。
