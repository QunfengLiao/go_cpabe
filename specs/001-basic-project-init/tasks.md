# 任务：基础工程初始化

**输入**：来自 `/specs/001-basic-project-init/` 的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`
**前置条件**：已完成规格和实现计划；本任务列表不包含登录、注册、权限、业务表、Docker Compose 数据服务、Kubernetes、CI/CD 或消息队列。

## 格式：`[ID] [P?] [Story?] 任务描述`

- **[P]**：可并行执行，前提是依赖任务已完成且修改文件互不冲突
- **[US1]**：配置 `.env` 并启动 Go 后端
- **[US2]**：连接远程 MySQL
- **[US3]**：连接远程 Redis
- **[US4]**：访问 `GET /health` 查看 app、mysql、redis 状态
- **[US5]**：启动 Electron + TypeScript 桌面端并看到基础首页
- **[US6]**：通过 README 完成本地启动、测试和排障

## 第 1 阶段：准备（初始化仓库基础文件）

**目标**：建立仓库级基础文件，避免真实凭据和本地产物进入版本控制。

- [ ] T001 初始化仓库忽略规则，覆盖 `.env`、依赖目录、构建产物、日志和临时文件，涉及 `.gitignore`
  - 输入：`plan.md` 的项目结构、`contracts/env.md` 的安全规则
  - 输出：根目录 `.gitignore`
  - 验收：运行 `git status --short` 时 `.env`、`node_modules/`、构建目录和日志文件不会作为待提交文件出现

- [ ] T002 [P] 创建示例环境变量文件，包含必需配置键且只使用占位值，涉及 `.env.example`
  - 输入：`contracts/env.md`
  - 输出：根目录 `.env.example`
  - 验收：文件包含 `APP_ENV`、`APP_PORT`、`MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_DATABASE`、`REDIS_ADDR`、`REDIS_PASSWORD`、`REDIS_DB`、`DESKTOP_API_BASE_URL`，且不包含真实服务器凭据

- [ ] T003 创建后端和桌面端顶层目录骨架，涉及 `backend/`、`desktop/`
  - 输入：`plan.md` 的推荐项目结构
  - 输出：`backend/` 和 `desktop/` 基础目录存在
  - 验收：运行 `find backend desktop -maxdepth 2 -type d` 能看到两个顶层工程目录

## 第 2 阶段：基础设施（所有用户故事的阻塞前置）

**目标**：建立后端和桌面端的最小工程骨架，后续任务只在既定结构内补充能力。

- [ ] T004 初始化 Go 模块和后端依赖声明，涉及 `backend/go.mod`、`backend/go.sum`
  - 输入：`plan.md` 技术上下文
  - 输出：Go 模块文件，依赖包含 Gin、Gorm、MySQL Driver、go-redis、godotenv
  - 验收：在 `backend/` 运行 `go mod tidy` 成功

- [ ] T005 创建后端分层目录和占位文件，涉及 `backend/cmd/server/`、`backend/internal/config/`、`backend/internal/router/`、`backend/internal/handler/`、`backend/internal/service/`、`backend/internal/repository/`、`backend/internal/model/`、`backend/internal/middleware/`、`backend/internal/pkg/`
  - 输入：`plan.md` 项目结构、宪章模块边界
  - 输出：后端目录结构与必要占位文件
  - 验收：目录存在，且未创建用户、文件、策略、权限等业务实现

- [ ] T006 [P] 初始化桌面端包配置和 TypeScript/Vite 基础配置，涉及 `desktop/package.json`、`desktop/tsconfig.json`、`desktop/vite.config.ts`、`desktop/index.html`
  - 输入：`plan.md` 技术上下文、`research.md` 决策 6
  - 输出：Electron + TypeScript + Vite 的基础配置文件
  - 验收：在 `desktop/` 运行 `npm install` 后可执行 `npm run typecheck`

## 第 3 阶段：用户故事 1 - 配置 `.env` 并启动 Go 后端（P1）

**目标**：开发者可以配置 `.env`，启动 Go 后端服务，并确认入口职责清晰。

**独立测试标准**：复制 `.env.example` 为 `.env` 后，在 `backend/` 运行 `go run ./cmd/server`，服务能按 `APP_PORT` 启动；`main.go` 不堆业务逻辑。

- [ ] T007 [US1] 实现后端配置结构和 `.env` 加载逻辑，涉及 `backend/internal/config/config.go`
  - 输入：`.env.example`、`contracts/env.md`
  - 输出：可读取 `APP_ENV`、`APP_PORT`、MySQL、Redis 配置的配置模块
  - 验收：缺失必填配置时返回明确错误；配置值不得在错误中泄露密码

- [ ] T008 [US1] 实现后端启动入口，保持 `main.go` 只负责加载配置、初始化依赖、注册路由和启动服务，涉及 `backend/cmd/server/main.go`
  - 输入：`plan.md` 阶段 A、T007 输出
  - 输出：最小可运行的 Go 服务入口
  - 验收：在 `backend/` 运行 `go run ./cmd/server` 能启动服务，入口文件不包含具体业务逻辑

- [ ] T009 [P] [US1] 增加后端配置加载单元测试，涉及 `backend/internal/config/config_test.go`
  - 输入：T007 输出、`contracts/env.md`
  - 输出：覆盖默认值、必填项缺失和数字解析失败的测试
  - 验收：在 `backend/` 运行 `go test ./...` 通过

## 第 4 阶段：用户故事 2 - 连接远程 MySQL（P1）

**目标**：开发者可以通过 `.env` 让后端连接远程 MySQL 8，并获得连接状态。

**独立测试标准**：填写远程 MySQL 配置后，后端能够初始化 Gorm 连接；配置错误时返回安全失败摘要。

- [ ] T010 [US2] 实现 Gorm MySQL 连接初始化模块，涉及 `backend/internal/repository/mysql.go`
  - 输入：T007 配置模块、`research.md` 决策 3
  - 输出：基于环境配置创建 Gorm MySQL 连接的函数
  - 验收：不硬编码 host、user、password、database；连接失败返回可分类错误

- [ ] T011 [P] [US2] 实现 MySQL 连通性检查封装，涉及 `backend/internal/service/health_mysql.go`
  - 输入：T010 输出、`contracts/health-api.md`
  - 输出：返回 `ok` 或 `error` 状态和脱敏摘要的 MySQL 检查逻辑
  - 验收：错误消息不包含 `MYSQL_PASSWORD`、完整 DSN 或真实敏感连接串

## 第 5 阶段：用户故事 3 - 连接远程 Redis（P1）

**目标**：开发者可以通过 `.env` 让后端连接远程 Redis 7，并获得连接状态。

**独立测试标准**：填写远程 Redis 配置后，后端能够初始化 Redis 客户端；配置错误时返回安全失败摘要。

- [ ] T012 [US3] 实现 go-redis 客户端初始化模块，涉及 `backend/internal/repository/redis.go`
  - 输入：T007 配置模块、`research.md` 决策 4
  - 输出：基于环境配置创建 Redis 客户端的函数
  - 验收：不硬编码 Redis 地址、密码或库编号；连接失败返回可分类错误

- [ ] T013 [P] [US3] 实现 Redis 连通性检查封装，涉及 `backend/internal/service/health_redis.go`
  - 输入：T012 输出、`contracts/health-api.md`
  - 输出：返回 `ok` 或 `error` 状态和脱敏摘要的 Redis 检查逻辑
  - 验收：错误消息不包含 `REDIS_PASSWORD`、访问令牌或真实敏感连接串

## 第 6 阶段：用户故事 4 - 实现健康检查接口（P1）

**目标**：开发者访问 `GET /health` 时，可以看到 app、mysql、redis 的基础状态。

**独立测试标准**：启动后端后访问 `GET /health`，响应符合 `contracts/health-api.md`；任一依赖异常时整体状态为 `degraded`。

- [ ] T014 [US4] 定义健康检查响应结构和状态聚合服务，涉及 `backend/internal/model/health.go`、`backend/internal/service/health.go`
  - 输入：`data-model.md` 健康检查结果、T011 输出、T013 输出
  - 输出：统一聚合 app、mysql、redis 状态的服务
  - 验收：全部可用时整体状态为 `ok`，任一依赖异常时整体状态为 `degraded`

- [ ] T015 [US4] 实现健康检查 Handler，涉及 `backend/internal/handler/health.go`
  - 输入：T014 输出、`contracts/health-api.md`
  - 输出：`GET /health` 的 HTTP 处理逻辑
  - 验收：响应 JSON 包含 `status`、`checkedAt`、`app`、`mysql`、`redis`

- [ ] T016 [US4] 集中注册后端路由，涉及 `backend/internal/router/router.go`
  - 输入：T015 输出
  - 输出：集中路由注册函数
  - 验收：`main.go` 通过 router 模块注册路由，不直接散落具体路由定义

- [ ] T017 [P] [US4] 增加健康检查接口测试，涉及 `backend/internal/handler/health_test.go`
  - 输入：T014、T015、T016 输出，`contracts/health-api.md`
  - 输出：覆盖全部可用和依赖异常的接口测试
  - 验收：在 `backend/` 运行 `go test ./...` 通过，响应不泄露密码字段值

## 第 7 阶段：用户故事 5 - 启动 Electron + TypeScript 桌面端（P2）

**目标**：开发者可以启动桌面端应用，看到基础首页和后端连接状态占位。

**独立测试标准**：在 `desktop/` 运行 `npm run typecheck`、`npm run build`、`npm run dev`，桌面端窗口能打开并展示基础首页。

- [ ] T018 [US5] 实现 Electron 主进程和预加载入口，涉及 `desktop/src/main/main.ts`、`desktop/src/preload/preload.ts`
  - 输入：T006 输出、`plan.md` 阶段 C
  - 输出：可启动窗口的 Electron 主进程和安全预加载入口
  - 验收：桌面端启动时能加载渲染页面，不引入复杂业务逻辑

- [ ] T019 [US5] 实现基础首页渲染入口和样式，涉及 `desktop/src/renderer/main.ts`、`desktop/src/renderer/App.ts`、`desktop/src/renderer/styles.css`
  - 输入：`data-model.md` 桌面端基础首页
  - 输出：展示项目名称和后端连接状态占位的基础首页
  - 验收：首页不依赖后端成功连接即可渲染；不引入复杂 UI 框架

- [ ] T020 [P] [US5] 补充桌面端开发、构建、类型检查脚本，涉及 `desktop/package.json`
  - 输入：T006、T018、T019 输出
  - 输出：`npm run dev`、`npm run build`、`npm run typecheck` 可用
  - 验收：在 `desktop/` 运行 `npm run typecheck` 和 `npm run build` 成功

## 第 8 阶段：用户故事 5 - 预留桌面端 API 调用结构（P2）

**目标**：开发者能在桌面端代码中看到统一 API 调用封装，后续可调用 Go 后端。

**独立测试标准**：`desktop/src/renderer/api` 存在，API 基础地址集中管理，健康检查调用封装可被首页后续接入。

- [ ] T021 [US5] 实现桌面端 API 配置模块，涉及 `desktop/src/renderer/api/config.ts`
  - 输入：`contracts/desktop-api.md`、`.env.example`
  - 输出：集中管理后端基础地址的配置模块
  - 验收：后端地址没有散落在页面组件中

- [ ] T022 [US5] 实现桌面端 HTTP 客户端和错误归一化，涉及 `desktop/src/renderer/api/client.ts`
  - 输入：`contracts/desktop-api.md`
  - 输出：统一请求函数和安全错误对象
  - 验收：网络失败时调用层能返回简短错误，不显示敏感配置

- [ ] T023 [P] [US5] 实现健康检查 API 封装，涉及 `desktop/src/renderer/api/health.ts`
  - 输入：T021、T022 输出，`contracts/health-api.md`
  - 输出：`getHealth` 调用封装和响应类型
  - 验收：类型定义覆盖 `app`、`mysql`、`redis` 状态字段

## 第 9 阶段：用户故事 6 - 补充 README 和启动说明（P2）

**目标**：新开发者可以通过 README 完成环境变量配置、后端启动、桌面端启动、测试运行和常见问题排查。

**独立测试标准**：仅阅读 README，开发者能按步骤完成 `quickstart.md` 中的验证流程。

- [ ] T024 [US6] 编写根目录 README，覆盖项目说明、技术栈、目录结构和范围边界，涉及 `README.md`
  - 输入：`spec.md`、`plan.md`、宪章
  - 输出：中文 README 的项目说明和范围说明
  - 验收：README 明确本阶段不实现登录、注册、权限、业务表和默认 MySQL/Redis 容器

- [ ] T025 [US6] 在 README 中补充环境变量、后端启动、桌面端启动和测试命令，涉及 `README.md`
  - 输入：`quickstart.md`、`contracts/env.md`
  - 输出：可复制执行的启动和测试说明
  - 验收：README 包含 `cp .env.example .env`、`go test ./...`、`go run ./cmd/server`、`npm run typecheck`、`npm run build`、`npm run dev`

- [ ] T026 [P] [US6] 在 README 中补充常见问题排查和敏感信息安全提醒，涉及 `README.md`
  - 输入：`quickstart.md` 常见问题、`contracts/env.md` 安全规则
  - 输出：配置缺失、网络不可达、认证失败、依赖未安装的排查说明
  - 验收：README 提醒不得提交真实 `.env`、个人服务器 IP、账号、密码或令牌

## 第 10 阶段：最终验证和清理

**目标**：确认基础工程可以按文档启动，任务范围未越界，仓库没有真实敏感信息。

- [ ] T027 运行后端格式化、依赖整理和测试，涉及 `backend/`
  - 输入：T004 到 T017 输出
  - 输出：格式化后的 Go 代码和干净依赖
  - 验收：在 `backend/` 运行 `gofmt -w .`、`go mod tidy`、`go test ./...` 成功

- [ ] T028 运行桌面端类型检查和构建验证，涉及 `desktop/`
  - 输入：T006、T018 到 T023 输出
  - 输出：通过类型检查和构建的桌面端工程
  - 验收：在 `desktop/` 运行 `npm run typecheck` 和 `npm run build` 成功

- [ ] T029 验证健康检查运行链路，涉及 `backend/`、`.env.example`
  - 输入：T007 到 T017 输出、远程 MySQL/Redis 本地 `.env`
  - 输出：`GET /health` 的实际响应
  - 验收：运行 `go run ./cmd/server` 后访问 `curl http://localhost:${APP_PORT:-8080}/health`，响应包含 app、mysql、redis 状态

- [ ] T030 检查范围边界和敏感信息，涉及 `.env.example`、`.gitignore`、`README.md`、`backend/`、`desktop/`
  - 输入：全部实现输出
  - 输出：范围和安全复核结果
  - 验收：不存在登录、注册、权限、业务表、Docker Compose 数据服务任务或代码；运行 `rg -n "password|secret|token|真实|服务器" .env.example README.md backend desktop` 后人工确认无真实凭据

## 依赖关系

```text
第 1 阶段 -> 第 2 阶段 -> US1 -> US2 + US3 -> US4 -> US5 -> US6 -> 最终验证
```

- US1 是后端启动和配置基础，必须先于 US2、US3、US4。
- US2 和 US3 可在 US1 后并行推进。
- US4 依赖 US2 和 US3 的状态检查封装。
- US5 可在第 2 阶段后与 US2/US3 部分并行，但最终验证需要后端健康检查完成。
- US6 可在主要工程结构稳定后编写，并在最终验证前完成。

## 并行执行示例

```bash
# 第 1 阶段后可并行
T002 创建 .env.example
T004 初始化 Go 模块
T006 初始化桌面端配置

# US1 后可并行
T011 实现 MySQL 连通性检查
T013 实现 Redis 连通性检查
T018 实现 Electron 主进程

# 桌面端阶段可并行
T020 补充桌面端脚本
T023 实现健康检查 API 封装
T026 补充 README 排障说明
```

## 实现策略

### MVP 优先

先完成第 1 阶段、第 2 阶段、US1、US2、US3 和 US4，得到可启动后端与 `GET /health`。这是最小可验证后端闭环。

### 增量交付

1. 后端骨架可启动。
2. `.env` 配置可读取。
3. MySQL 和 Redis 连接可诊断。
4. 健康检查接口可访问。
5. 桌面端可启动并展示基础首页。
6. API 封装和 README 补齐。
7. 执行最终验证和敏感信息复核。

### 范围纪律

- 不实现用户注册、登录、权限系统。
- 不实现业务表、数据库迁移或种子数据。
- 不默认生成 MySQL 或 Redis Docker Compose 服务。
- 不实现 CP-ABE、RSA、AES-GCM、策略解析、Benchmark 或 Audit 业务能力，只保留后续工程边界。
