# 任务：用户认证与资料基础模块

**输入**：来自 `specs/001-user-auth-profile/` 的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`
**前置条件**：`spec.md`、`plan.md`
**测试策略**：规格和 quickstart 要求接口与端到端验证，因此为每个用户故事生成测试优先任务。

## 格式：`[ID] [P?] [Story?] 任务描述`

- **[P]**：可并行执行，前提是依赖已完成且不修改同一文件。
- **[US1]**：用户注册与登录。
- **[US2]**：刷新 Token 与退出登录。
- **[US3]**：当前用户资料、资料编辑与头像上传。

## 第 1 阶段：准备

**目标**：创建后端工程骨架、依赖、配置和数据库迁移基础，使后续故事能在统一结构中实现。

- [X] T001 创建后端 Go module 和基础目录结构于 `backend/go.mod`
- [X] T002 [P] 创建后端启动入口占位于 `backend/cmd/server/main.go`
- [X] T003 [P] 创建配置结构和环境变量读取入口于 `backend/internal/config/config.go`
- [X] T004 [P] 创建本地开发配置示例于 `backend/.env.example`
- [X] T005 创建数据库连接初始化于 `backend/internal/config/database.go`
- [X] T006 创建 Redis 连接初始化于 `backend/internal/config/redis.go`
- [X] T007 [P] 创建统一响应结构于 `backend/internal/pkg/response/response.go`
- [X] T008 [P] 创建统一错误码定义于 `backend/internal/pkg/response/errors.go`
- [X] T009 创建路由注册入口于 `backend/internal/handler/router.go`
- [X] T010 创建 `users` 表 SQL migration 于 `backend/migrations/001_create_users.sql`

## 第 2 阶段：基础能力

**目标**：完成所有用户故事共用的领域模型、DTO、Repository、Token、Redis Store、Storage 和中间件基础。

- [X] T011 [P] 定义用户角色、状态和 Token 类型常量于 `backend/internal/domain/constants.go`
- [X] T012 [P] 定义 User Gorm Model 于 `backend/internal/domain/user.go`
- [X] T013 [P] 定义 UserDTO 和 DTO 映射函数于 `backend/internal/domain/user_dto.go`
- [X] T014 [P] 定义 Repository 接口和 Gorm 实现骨架于 `backend/internal/repository/user_repository.go`
- [X] T015 实现按邮箱查询、按 ID 查询、创建用户、更新资料、更新头像方法于 `backend/internal/repository/user_repository.go`
- [X] T016 [P] 定义 Auth Claims、TokenPair 和 RefreshSession 类型于 `backend/internal/pkg/auth/types.go`
- [X] T017 [P] 实现密码 bcrypt Hash 与校验于 `backend/internal/pkg/auth/password.go`
- [X] T018 实现 Access Token 生成、解析和 `token_type=access` 校验于 `backend/internal/pkg/auth/jwt.go`
- [X] T019 实现 Refresh Token 随机生成、Hash、`token_id` 与 `session_id` 工具于 `backend/internal/pkg/auth/refresh.go`
- [X] T020 [P] 定义 Redis Token Store 接口和 Key 规则于 `backend/internal/pkg/auth/token_store.go`
- [X] T021 实现 Redis Refresh Session 保存、查询、删除和 TTL 设置于 `backend/internal/pkg/auth/redis_token_store.go`
- [X] T022 实现 Refresh Token 轮换能力于 `backend/internal/pkg/auth/redis_token_store.go`
- [X] T023 [P] 定义 Storage 接口和上传结果类型于 `backend/internal/pkg/storage/storage.go`
- [X] T024 实现本地头像存储、目录创建、文件名生成和 URL 拼接于 `backend/internal/pkg/storage/local_storage.go`
- [X] T025 [P] 创建请求参数校验辅助函数于 `backend/internal/pkg/validator/validator.go`
- [X] T026 实现 Access Token 认证中间件并注入 `user_id`、`role` 于 `backend/internal/middleware/auth.go`

## 第 3 阶段：用户故事 1 - 注册与登录

**目标**：新用户可以注册 `data_owner` 或 `data_user`，禁止公开创建 `admin`，已注册用户可以邮箱密码登录并获得双 Token。

**独立测试标准**：可以注册 `data_owner` 和 `data_user`；注册 `admin`、重复邮箱、密码不一致失败；正确密码登录成功并写入 Redis；错误密码返回统一错误；响应不包含 `password_hash` 和 `avatar_object_key`。

- [X] T027 [P] [US1] 编写注册接口成功与失败测试于 `backend/internal/handler/auth_register_test.go`
- [X] T028 [P] [US1] 编写登录接口成功、统一失败和 Redis 写入失败测试于 `backend/internal/handler/auth_login_test.go`
- [X] T029 [P] [US1] 编写用户服务注册规则测试于 `backend/internal/service/auth_register_test.go`
- [X] T030 [P] [US1] 编写用户服务登录规则测试于 `backend/internal/service/auth_login_test.go`
- [X] T031 [US1] 定义注册、登录请求和响应结构于 `backend/internal/handler/auth_handler.go`
- [X] T032 [US1] 实现 AuthService 注册流程于 `backend/internal/service/auth_service.go`
- [X] T033 [US1] 实现 AuthService 登录流程和 Refresh Session 创建于 `backend/internal/service/auth_service.go`
- [X] T034 [US1] 实现注册 Handler 并映射统一错误响应于 `backend/internal/handler/auth_handler.go`
- [X] T035 [US1] 实现登录 Handler 并返回 Access Token、Refresh Token 和 UserDTO 于 `backend/internal/handler/auth_handler.go`
- [X] T036 [US1] 注册 `/api/v1/auth/register` 和 `/api/v1/auth/login` 路由于 `backend/internal/handler/router.go`

## 第 4 阶段：用户故事 2 - 刷新 Token 与退出登录

**目标**：用户可以使用有效 Refresh Token 刷新 Access Token，默认执行 Refresh Token 轮换；退出当前会话后旧 Refresh Token 失效。

**独立测试标准**：有效 Refresh Token 可刷新；Access Token 冒充 Refresh Token 失败；Redis 登录态不存在、Hash 不匹配、过期 Token 失败；轮换后旧 Refresh Token 失败；退出后旧 Refresh Token 不能刷新。

- [X] T037 [P] [US2] 编写刷新 Token 成功、轮换和失败测试于 `backend/internal/handler/auth_refresh_test.go`
- [X] T038 [P] [US2] 编写退出登录成功和幂等失败场景测试于 `backend/internal/handler/auth_logout_test.go`
- [X] T039 [P] [US2] 编写 Redis Token Store 轮换测试于 `backend/internal/pkg/auth/redis_token_store_test.go`
- [X] T040 [US2] 定义刷新和退出请求响应结构于 `backend/internal/handler/auth_handler.go`
- [X] T041 [US2] 实现 AuthService 刷新 Token 流程和用户状态校验于 `backend/internal/service/auth_service.go`
- [X] T042 [US2] 实现 AuthService 退出登录流程于 `backend/internal/service/auth_service.go`
- [X] T043 [US2] 实现刷新 Handler 并拒绝缺失、无效和不匹配 Refresh Token 于 `backend/internal/handler/auth_handler.go`
- [X] T044 [US2] 实现退出 Handler 并删除当前 Refresh Session 于 `backend/internal/handler/auth_handler.go`
- [X] T045 [US2] 注册 `/api/v1/auth/refresh` 和 `/api/v1/auth/logout` 路由于 `backend/internal/handler/router.go`

## 第 5 阶段：用户故事 3 - 当前用户资料与头像

**目标**：登录用户可以使用 Access Token 获取当前用户资料，编辑昵称、简介、生日，并上传合法头像。

**独立测试标准**：有效 Access Token 可访问 `/users/me`；缺失、无效、过期 Access Token 和 Refresh Token 均被拒绝；资料编辑只能修改允许字段；头像空文件、不支持类型、超过 2MB 失败；合法头像上传后资料显示新 `avatar_url`。

- [X] T046 [P] [US3] 编写认证中间件 Access Token 与 Refresh Token 混用测试于 `backend/internal/middleware/auth_test.go`
- [X] T047 [P] [US3] 编写获取当前用户接口测试于 `backend/internal/handler/user_me_test.go`
- [X] T048 [P] [US3] 编写编辑当前用户资料接口测试于 `backend/internal/handler/user_update_test.go`
- [X] T049 [P] [US3] 编写头像上传接口成功和失败测试于 `backend/internal/handler/user_avatar_test.go`
- [X] T050 [P] [US3] 编写本地头像存储测试于 `backend/internal/pkg/storage/local_storage_test.go`
- [X] T051 [US3] 定义当前用户、资料编辑和头像上传请求响应结构于 `backend/internal/handler/user_handler.go`
- [X] T052 [US3] 实现 UserService 当前用户查询和 disabled 状态处理于 `backend/internal/service/user_service.go`
- [X] T053 [US3] 实现 UserService 资料编辑规则于 `backend/internal/service/user_service.go`
- [X] T054 [US3] 实现 UserService 头像上传业务流程和旧头像删除预留点于 `backend/internal/service/user_service.go`
- [X] T055 [US3] 实现获取当前用户 Handler 于 `backend/internal/handler/user_handler.go`
- [X] T056 [US3] 实现编辑当前用户资料 Handler 于 `backend/internal/handler/user_handler.go`
- [X] T057 [US3] 实现头像上传 Handler 的 multipart、大小和类型校验于 `backend/internal/handler/user_handler.go`
- [X] T058 [US3] 注册受保护用户路由和认证中间件组合于 `backend/internal/handler/router.go`

## 第 6 阶段：打磨与横切验证

**目标**：完成配置、忽略文件、集成验证和文档同步，确保实现符合规格与宪章边界。

- [X] T059 [P] 补齐 Go、Node、环境变量和通用忽略规则于 `.gitignore`
- [X] T060 [P] 编写 quickstart 端到端验证脚本于 `backend/scripts/verify_auth_flow.sh`
- [X] T061 [P] 增加后端 README 使用说明于 `backend/README.md`
- [X] T062 运行 Go 格式化和静态检查并修复问题于 `backend/`
- [X] T063 运行 `go test ./...` 并记录验证结果于 `specs/001-user-auth-profile/quickstart.md`
- [X] T064 检查所有用户响应不包含 `password_hash` 和 `avatar_object_key` 于 `backend/internal/handler/`
- [X] T065 检查本功能未引入文件加密、CP-ABE 模拟、Benchmark 结论或完整 RBAC 于 `specs/001-user-auth-profile/plan.md`

## 依赖关系

- 第 1 阶段必须先完成，为后端目录、配置、响应和 migration 提供基础。
- 第 2 阶段阻塞所有用户故事，必须在 US1、US2、US3 前完成。
- US1 是 MVP，提供用户、登录和 Token 基础。
- US2 依赖 US1 的登录和 Refresh Session 创建能力。
- US3 依赖 US1 的 Access Token 和认证中间件能力；头像存储基础来自第 2 阶段。
- 第 6 阶段在所有用户故事完成后执行。

## 并行执行示例

### US1 并行

```text
T027、T028、T029、T030 可并行编写测试。
T034 和 T035 修改同一 Handler 文件，必须顺序执行。
```

### US2 并行

```text
T037、T038、T039 可并行编写测试。
T041 和 T042 修改同一 Service 文件，必须顺序执行。
```

### US3 并行

```text
T046、T047、T048、T049、T050 可并行编写测试。
T052、T053、T054 修改同一 Service 文件，必须顺序执行。
T055、T056、T057 修改同一 Handler 文件，必须顺序执行。
```

## 实施策略

### MVP 优先

先完成第 1 阶段、第 2 阶段和 US1。完成后应能独立验证注册、登录、双 Token 签发、Redis Refresh Session 写入和敏感字段不泄露。

### 增量交付

1. MVP：第 1 阶段 + 第 2 阶段 + US1。
2. 会话闭环：增加 US2，完成刷新与退出。
3. 资料闭环：增加 US3，完成当前用户资料、资料编辑和头像上传。
4. 打磨：执行第 6 阶段，完成 quickstart 验证和文档同步。

### 宪章边界

本任务清单只实现 User 模块基础能力，不实现文件加密、访问树、LSSS、CP-ABE、RSA+AES、Benchmark 或完整 RBAC。
