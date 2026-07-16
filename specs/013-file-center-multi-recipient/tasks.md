# 任务：多接收者文件中心与本地解密闭环

**输入**：`specs/013-file-center-multi-recipient/` 下的 `spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`  
**前置条件**：已确认本功能按 RSA+AES 多接收者主链路实施，CP-ABE 扩展点保留但不模拟实现。  
**测试要求**：本功能涉及核心业务模型和安全边界，所有变更按测试优先执行；测试任务必须先写并确认失败，再进入对应实现任务。  
**组织方式**：按用户故事拆分，保证每个故事可独立验证。

## 阶段 1：准备与基线

**目标**：固定当前单接收者问题和验证命令，避免后续改动失去回归基准。

- [x] T001 记录当前后端测试环境和 Go SDK 可用性，确认 `go test` 执行路径或 GoLand SDK 路径
- [x] T002 [P] 运行桌面端现有测试并记录基线：`desktop/src/main/encryption/coordinator.test.ts`、`desktop/src/renderer/src/pages/DataEncryptionPage.test.tsx`
- [x] T003 [P] 运行桌面端类型检查 `desktop/tsconfig.json`
- [x] T004 [P] 检查当前数据库 011 迁移约束并记录 `protected_keys`、`rsa_protected_key_bindings`、`encryption_benchmarks` 的唯一键现状

---

## 阶段 2：基础数据结构和契约

**目标**：完成阻塞所有故事的一对多 protected DEK 结构、DTO 和契约测试。

- [x] T005 [P] 编写迁移结构测试，验证 012 迁移移除单 protected DEK 唯一约束并增加接收者唯一约束：`backend/internal/migrations/encryption_framework_test.go`
- [x] T006 新增数据库迁移 `backend/migrations/012_multi_recipient_file_center.sql`，调整 `protected_keys`、`rsa_protected_key_bindings`、`encryption_benchmarks` 和执行进度字段
- [x] T007 修改 SQL 迁移装载顺序，加入 012 迁移：`backend/internal/migrations/sql_migrations.go`
- [x] T008 [P] 编写 Domain 结构测试，验证 ProtectedKey 与 RSA 绑定支持一文件多记录和接收者耗时字段：`backend/internal/domain/encryption_algorithm_independence_test.go`
- [x] T009 修改领域模型字段和注释：`backend/internal/domain/encryption_key.go`、`backend/internal/domain/encryption_task.go`
- [x] T010 [P] 编写桌面端类型测试，验证多接收者 descriptor、protected key 数组和进度事件字段：`desktop/src/main/encryption/cryptoWorkerProtocol.test.ts`
- [x] T011 修改桌面端加密类型契约：`desktop/src/main/encryption/types.ts`、`desktop/src/renderer/src/vite-env.d.ts`

**检查点**：数据模型可以表达“一个文件、一份密文、多份 protected DEK”。

---

## 阶段 3：用户故事 1 - DO 将一个文件安全分享给多个接收者（P1）MVP

**目标**：后端和本地 worker 支持多接收者加密，完成一份密文、多份 protected DEK 入库。

**独立测试**：创建一个三接收者加密任务并完成，验证只有一份密文对象，存在 owner + 3 名接收者的 protected DEK，重复接收者和跨租户公钥被拒绝。

### 测试

- [x] T012 [P] [US1] 编写后端服务失败测试：重复接收者、公钥不属于用户、缺少 owner 授权时拒绝任务：`backend/internal/service/encryption_service_test.go`
- [x] T013 [P] [US1] 编写后端仓储完成测试：一次完成写入多条 protected DEK 和多条 RSA binding：`backend/internal/repository/encryption_repository_test.go`
- [x] T014 [P] [US1] 编写 Handler 契约测试：`authorization.recipients[]` 创建任务，旧单接收者请求返回兼容错误或被规范化拒绝：`backend/internal/handler/encryption_handler_test.go`
- [x] T015 [P] [US1] 编写 Crypto worker e2e 测试：同一密文、同一 DEK 对多个 RSA 公钥分别封装并可各自解封：`desktop/src/main/encryption/encryption.e2e.test.ts`

### 实现

- [x] T016 [US1] 修改 RSA 授权适配器支持 `RSA_RECIPIENTS` 快照和逐接收者校验：`backend/internal/service/encryption_algorithm_adapter.go`
- [x] T017 [US1] 修改创建任务输入和任务创建逻辑，冻结接收者快照并记录接收者总数：`backend/internal/service/encryption_service.go`
- [x] T018 [US1] 修改完成任务输入和校验逻辑，按快照校验 `protected_keys[]`：`backend/internal/service/encryption_service.go`
- [x] T019 [US1] 修改仓储完成事务，一次提交文件、密文对象、多 protected DEK、多 RSA binding 和性能指标：`backend/internal/repository/encryption_repository.go`
- [x] T020 [US1] 修改 Handler 请求绑定和响应 DTO，返回多接收者授权摘要：`backend/internal/handler/encryption_handler.go`
- [x] T021 [US1] 修改 RSA key 查询服务，支持 owner 最新有效公钥和每用户历史有效公钥版本：`backend/internal/service/rsa_key_service.go`、`backend/internal/repository/rsa_key_repository.go`
- [x] T022 [US1] 修改本地 Crypto 引擎请求和结果，支持 `Authorizations[]` 与 `ProtectedKeys[]`：`backend/internal/crypto/content_cipher.go`
- [x] T023 [US1] 修改 RSA 引擎保持单次 DEK 保护能力，并为多接收者调用提供耗时结果：`backend/internal/crypto/rsa_engine.go`
- [x] T024 [US1] 修改 crypto-worker JSON 协议，返回 protected key 数组：`backend/cmd/crypto-worker/main.go`
- [x] T025 [US1] 修改 Electron 协调器创建任务、调用 worker 和完成提交的多接收者流程：`desktop/src/main/encryption/coordinator.ts`
- [x] T026 [US1] 修改任务 API 客户端类型和完成请求：`desktop/src/main/encryption/taskApiClient.ts`

**检查点**：无需重构页面也可通过测试证明多接收者业务模型正确。

---

## 阶段 4：用户故事 2 - 文件中心按授权状态查看和操作文件（P1）

**目标**：新增统一文件中心 scope 查询、详情和下载鉴权，保留旧路由兼容。

**独立测试**：文件可见用户在企业云盘可见元数据、下载密文并获取密钥信封；没有匹配本地私钥时不生成明文且保留密文包；分享给我可按目标公钥信封辅助筛选文件。

### 测试

- [x] T027 [P] [US2] 编写仓储查询测试，覆盖 `tenant_cloud`、`shared_with_me`、`owned_by_me` 三种 scope：`backend/internal/repository/encrypted_file_query_test.go`
- [x] T028 [P] [US2] 编写服务鉴权测试，验证未授权用户不能获取解密材料但可下载企业云盘密文：`backend/internal/service/encrypted_file_service_test.go`
- [x] T029 [P] [US2] 编写 Handler 路由测试，验证 `/tenant/files?scope=...`、详情、密文下载、解密材料接口：`backend/internal/handler/encrypted_file_handler_test.go`

### 实现

- [x] T030 [US2] 新增文件中心聚合 DTO 和查询结果结构：`backend/internal/repository/encryption_repository.go`
- [x] T031 [US2] 实现统一 scope 查询、owner 摘要、接收者摘要和授权状态计算：`backend/internal/repository/encryption_repository.go`
- [x] T032 [US2] 实现文件中心服务方法，区分文件可见、密文可下载和本地解密尝试：`backend/internal/service/encrypted_file_service.go`
- [x] T033 [US2] 实现文件中心 Handler 和兼容旧接口响应：`backend/internal/handler/encrypted_file_handler.go`
- [x] T034 [US2] 注册新路由并保留旧 `encrypted-files`、`received-files` 路由兼容：`backend/internal/handler/router.go`
- [x] T035 [US2] 修改前端 encryption API，新增 `listFileCenterItems`、`getFileCenterDetail`、`downloadFileCiphertext`、`getOwnDecryptionMaterial`：`desktop/src/renderer/src/api/encryption.ts`

**检查点**：不依赖新 UI，也能用接口和测试验证三种文件范围与授权边界。

---

## 阶段 5：用户故事 3 - 授权用户选择输出目录并安全完成本地解密（P1）

**目标**：将本地解密改为选择目录、自动重命名、安全打开所在文件夹，并防止失败残留明文。

**独立测试**：授权用户选择目录解密成功；同名文件自动编号；RSA/AES 失败不保留部分明文；取消目录选择不标记远程失败。

### 测试

- [x] T036 [P] [US3] 编写主进程解密测试，验证选择目录、自动重命名和 reveal token：`desktop/src/main/encryption/decryptionCoordinator.test.ts`
- [x] T037 [P] [US3] 编写安全测试，验证解密失败清理 `.part-*` 文件且不调用打开文件夹：`desktop/src/main/encryption/security.test.ts`
- [x] T038 [P] [US3] 编写 preload 类型测试，验证 Renderer 只能调用受限解密和 reveal API：`desktop/src/main/encryption/ipcValidation.test.ts`

### 实现

- [x] T039 [US3] 修改主进程解密协调器，使用文件夹选择、临时密文、自动重命名和成功后打开所在文件夹：`desktop/src/main/encryption/decryptionCoordinator.ts`
- [x] T040 [US3] 新增或修改安全文件名与不重名路径工具：`desktop/src/main/encryption/decryptionCoordinator.ts`
- [x] T041 [US3] 增加 reveal token 管理并在清理上下文时释放：`desktop/src/main/encryption/decryptionCoordinator.ts`、`desktop/src/main/main.ts`
- [x] T042 [US3] 修改 preload 暴露 `decryptFile` 与 `revealDecryptedFile`，移除或兼容旧 `decryptReceivedFile`：`desktop/src/preload/preload.ts`
- [x] T043 [US3] 修改 Renderer 类型声明和调用点：`desktop/src/renderer/src/vite-env.d.ts`

**检查点**：授权用户可以安全解密到指定目录，失败不产生伪明文。

---

## 阶段 6：用户故事 4 - 真实进度和性能指标（P2）

**目标**：补齐真实进度、分项耗时、接收者级 RSA 指标和详情展示数据。

**独立测试**：进度分别来自字节、接收者计数和上传字节；性能指标按任务级与接收者级入库并可查询。

### 测试

- [x] T044 [P] [US4] 编写进度桥测试，验证总体百分比单调递增且不使用随机进度：`desktop/src/main/encryption/progressBridge.test.ts`
- [x] T045 [P] [US4] 编写 benchmark 测试，验证耗时字段、接收者数量和 protected key 总大小计算：`desktop/src/main/encryption/benchmark.test.ts`
- [x] T046 [P] [US4] 编写后端 benchmark 入库和详情查询测试：`backend/internal/service/audit_encryption_test.go`、`backend/internal/repository/encryption_repository_test.go`

### 实现

- [x] T047 [US4] 扩展 worker 进度，增加接收者保护计数和阶段耗时：`backend/internal/crypto/content_cipher.go`、`backend/cmd/crypto-worker/main.go`
- [x] T048 [US4] 扩展 Electron 进度桥，组合真实阶段进度并保持百分比单调：`desktop/src/main/encryption/progressBridge.ts`
- [x] T049 [US4] 修改上传客户端，支持上传字节进度或基于流读取统计上传字节：`desktop/src/main/encryption/taskApiClient.ts`
- [x] T050 [US4] 修改 benchmark 采集，使用 `performance.now()` 或单调时钟记录验证、AES、DEK 保护、上传、元数据提交和总耗时：`desktop/src/main/encryption/coordinator.ts`
- [x] T051 [US4] 修改后端 benchmark domain、repository 和 service，保存任务级与接收者级指标：`backend/internal/domain/encryption_task.go`、`backend/internal/repository/encryption_repository.go`、`backend/internal/service/encryption_service.go`
- [x] T052 [US4] 修改审计元数据白名单，允许新增非敏感 benchmark 标量并拒绝密钥材料：`backend/internal/repository/audit_repository.go`

**检查点**：进度和性能指标可作为 RSA 与后续 CP-ABE 对比依据。

---

## 阶段 7：用户故事 5 - 页面向导、文件中心和详情抽屉（P3）

**目标**：完成紧凑三步加密向导、文件中心三页签、统一格式化和分组详情抽屉。

**独立测试**：用户可在新向导中选择文件、多接收者和公钥版本；文件中心按页签显示中文状态和操作；详情不显示数字接收者 ID。

### 测试

- [x] T053 [P] [US5] 编写格式化工具测试，覆盖 B/KiB/MiB/GiB、中文状态和 `YYYY-MM-DD HH:mm:ss`：`desktop/src/renderer/src/utils/format.test.ts`
- [x] T054 [P] [US5] 编写数据加密页测试，验证多选接收者、owner 锁定、确认摘要和按钮文案：`desktop/src/renderer/src/pages/DataEncryptionPage.test.tsx`
- [x] T055 [P] [US5] 编写文件中心页测试，验证三个页签、授权状态和未授权解密提示：`desktop/src/renderer/src/pages/FileCenterPage.test.tsx`
- [x] T056 [P] [US5] 编写详情抽屉测试，验证接收者列表、性能分组、哈希缩短和复制入口：`desktop/src/renderer/src/components/encryption/EncryptedFileDetail.test.tsx`

### 实现

- [x] T057 [P] [US5] 新增统一格式化工具：`desktop/src/renderer/src/utils/format.ts`
- [x] T058 [US5] 重构文件选择组件，支持紧凑展示、动态大小、替换、移除和校验状态：`desktop/src/renderer/src/components/encryption/FileSelector.tsx`
- [x] T059 [US5] 重构算法和接收者选择组件为多选列表、搜索、每用户公钥版本选择和 owner 锁定：`desktop/src/renderer/src/components/encryption/AlgorithmAuthorizationForm.tsx`
- [x] T060 [US5] 重构确认组件，展示接收者数量、姓名、公钥版本和安全提示：`desktop/src/renderer/src/components/encryption/EncryptionConfirmation.tsx`
- [x] T061 [US5] 重构进度组件，展示六步步骤条、线性进度、字节、耗时、接收者进度和取消语义：`desktop/src/renderer/src/components/encryption/EncryptionProgress.tsx`
- [x] T062 [US5] 重构数据加密页，移除长期占位 RSA 生成按钮，接入多接收者 descriptor：`desktop/src/renderer/src/pages/DataEncryptionPage.tsx`
- [x] T063 [US5] 新增文件中心页面和三页签：`desktop/src/renderer/src/pages/FileCenterPage.tsx`
- [x] T064 [US5] 将旧“我的加密文件”和“收到的文件”页面改为兼容跳转或复用文件中心对应页签：`desktop/src/renderer/src/pages/EncryptedFilesPage.tsx`、`desktop/src/renderer/src/pages/ReceivedFilesPage.tsx`
- [x] T065 [US5] 重构详情抽屉为基本信息、加密信息、授权接收者、性能指标、完整性信息五组：`desktop/src/renderer/src/components/encryption/EncryptedFileDetail.tsx`
- [x] T066 [US5] 更新菜单和路由，数据安全下保留“我的密钥”“数据加密”“文件中心”：`desktop/src/renderer/src/components/AppLayout.tsx`、`desktop/src/renderer/src/main.tsx`

**检查点**：页面交互与业务模型一致，不再展示 ISO 时间、英文状态或纯数字接收者 ID。

---

## 阶段 8：审计 Outbox 可靠投递（横切能力）

**目标**：将安全关键审计与业务事务原子入队，通过独立 Dispatcher 幂等投递到 `audit_logs`，并与文件孤儿清理边界彻底隔离。

**编号说明**：本阶段是在既有任务确认后增量加入，因此从 T075 继续编号；任务执行顺序以阶段依赖为准，不回写已存在任务编号。

### 测试

- [x] T075 [P] 编写 014 迁移静态测试，验证 `audit_outbox` 字段、事件 UUID/业务去重唯一键、领取索引、租约索引和幂等 DDL，且不复用 `orphan_storage_objects`：`backend/internal/migrations/encryption_framework_test.go`
- [x] T076 [P] 编写审计事件规范化测试，按 action 校验枚举、长度、数值范围、UUID/SHA-256、JSON 大小和敏感字段；非法 Metadata 只能产生 `metadata_redacted` 最小事件：`backend/internal/repository/audit_repository_test.go`
- [x] T077 [P] 编写 outbox 仓储测试，覆盖相同 `event_public_id` 和 `dedup_key` 幂等入队、平台事件空租户、业务事务回滚时 outbox 同时回滚：`backend/internal/repository/audit_outbox_repository_test.go`
- [x] T078 [P] 编写 Dispatcher 状态机测试，覆盖 `PENDING/RETRY -> PROCESSING -> DELIVERED`、指数退避、最大重试、`DEAD_LETTER` 和上下文取消：`backend/internal/service/audit_dispatcher_service_test.go`
- [x] T079 [P] 编写崩溃窗口测试，验证租约过期重新领取、旧 `lock_token` 不能提交结果、正式日志已写但状态更新失败后重放仍只有一条：`backend/internal/service/audit_dispatcher_service_test.go`
- [x] T080 [P] 编写两个 Dispatcher 并发领取测试，验证 `SKIP LOCKED` 不重复处理同一行且不同事件可并行推进：`backend/internal/repository/audit_outbox_repository_test.go`
- [x] T081 [P] 使用 `TEST_MYSQL_DSN` 编写集成测试，重复执行完整迁移并验证真实列/索引、事务入队、领取、投递、唯一冲突收敛和租约恢复：`backend/internal/migrations/audit_outbox_integration_test.go`
- [x] T082 [P] 编写加密主链路故障注入测试，验证创建/完成与 outbox 同事务、独立审计入队失败的安全降级，以及日志不含 Metadata、路径、密钥或原始数据库错误：`backend/internal/service/audit_encryption_test.go`

### 实现

- [x] T083 新增幂等迁移 `backend/migrations/014_audit_outbox.sql`，创建独立 outbox 表、事件 UUID 与业务去重唯一键、领取/租约/租户运维索引，并为 `audit_logs` 增加 `metadata_redacted` 字段
- [x] T084 接入 014 显式迁移、AutoMigrate 和后置校验：`backend/internal/migrations/sql_migrations.go`、`backend/internal/migrations/automigrate.go`、`backend/internal/migrations/encryption_validation.go`
- [x] T085 定义 `AuditOutboxEvent`、状态常量和全部字段中文安全注释，确保实体不经 JSON/API 暴露：`backend/internal/domain/audit.go`
- [x] T086 抽取审计 Envelope 规范化器，按 action 共用正式日志与 outbox 校验规则；生成稳定 `event_public_id`、可选 `dedup_key` 和 `occurred_at`：`backend/internal/service/audit_recorder.go`、`backend/internal/repository/audit_repository.go`
- [x] T087 实现事务感知 outbox 仓储、批量领取、租约 CAS、幂等正式投递、退避、死信和已投递保留期清理：`backend/internal/repository/audit_outbox_repository.go`
- [x] T088 将加密任务创建和完成等安全关键业务写操作改为业务事实与 outbox 同事务；无业务写事务事件使用独立短事务入队并保留最小结构化告警：`backend/internal/repository/encryption_repository.go`、`backend/internal/service/encryption_service.go`
- [x] T089 实现独立 `AuditDispatcherService`，单条失败不阻断批次其他事件，使用稳定错误分类且不记录原始错误文本：`backend/internal/service/audit_dispatcher_service.go`
- [x] T090 新增一次性/可调度 Dispatcher 命令，支持批次、租约、最大重试、退避上限、优雅退出和仅含稳定错误码的日志：`backend/cmd/audit-dispatcher/main.go`、`backend/internal/config/config.go`
- [x] T091 更新后端运行说明，记录 Dispatcher 调度、监控指标、已投递保留期、死信人工重放和“MySQL 整体不可用时不能承诺零丢失”的边界：`backend/README.md`
- [x] T092 执行审计 Outbox 关键注释和可读性检查，确认所有函数/方法有中文前置注释、导出标识符符合 GoDoc、实体字段说明敏感性与外部响应边界、事务/租约/幂等/死信注释解释设计原因且无无意义逐行注释

**检查点**：MySQL 可用时，安全关键业务事实与 outbox 同时提交；Dispatcher 至少一次投递最终收敛为一条正式日志，文件清理程序永不处理审计事件。

---

## 最终阶段：质量、注释和验收

**目标**：统一验证安全、测试、构建和文档交付。

- [x] T067 [P] 更新 README 或功能说明，描述文件中心、多接收者 RSA+AES 和本地解密安全边界：`backend/README.md`、`desktop/README.md`
- [x] T068 执行 Go 关键注释和可读性检查，确认新增/修改 Go 函数方法前置注释、GoDoc 前缀、实体字段注释、Handler/Service/Repository 安全边界注释均完成
- [x] T069 执行后端测试：`go test ./internal/domain ./internal/repository ./internal/service ./internal/handler ./internal/crypto`
- [x] T070 执行桌面端测试：`npm.cmd test -- --run`
- [x] T071 执行桌面端类型检查和构建：`npm.cmd run typecheck`、`npm.cmd run build`
- [ ] T072 按 `specs/013-file-center-multi-recipient/quickstart.md` 手工验证八个验收场景并记录结果
- [x] T073 [US4] 编写上传尾部进度乱序回归测试，验证队列清空前不得推进到元数据保存阶段：`desktop/src/main/encryption/coordinator.test.ts`
- [x] T074 [US4] 为 Electron 协调器增加串行进度队列，并在上传结束后清空队列再上报 `SAVING_METADATA`：`desktop/src/main/encryption/coordinator.ts`

---

## 修订阶段：分离 RBAC 可见性与本地解密

**目标**：落实“RBAC 不控制最终解密”的要求，确保同租户可见用户可以下载完整密文包和密钥信封，由客户端本地私钥决定是否能够恢复明文。

- [x] T093 [US2] 编写后端回归测试，验证缺少 `file.decrypt.invoke` 的同租户可见用户仍可获取文件详情、密文流和完整密钥信封集合，跨租户仍被拒绝：`backend/internal/handler/*_test.go`、`backend/internal/service/*_test.go`
- [x] T094 [US2] 扩展文件解密材料 DTO 与 Repository 聚合，返回 `key_id`、公钥指纹、受保护 DEK、上下文摘要和算法信息的完整信封集合，不返回私钥或明文 DEK：`backend/internal/repository/encryption_repository.go`、`backend/internal/service/encrypted_file_service.go`
- [x] T095 [US2] 移除 `file.decrypt.invoke` 对密文下载、解密材料和兼容收到文件路由的中间件依赖，保留 RBAC 在文件查看、上传、分享、公钥管理和审计管理中的职责：`backend/internal/handler/router.go`、`backend/migrations/013_do_received_file_permission.sql`
- [x] T096 [US2] 编写 Electron 解密回归测试，验证主进程按密钥信封 `key_id/公钥指纹` 遍历本地安全存储私钥，服务端无匹配信封或解封失败时保留原始密文包且不留下明文：`desktop/src/main/encryption/decryptionCoordinator.test.ts`、`desktop/src/main/encryption/security.test.ts`
- [x] T097 [US2] 修改 Electron 主进程解密协调器和 RSA 本地密钥索引，支持信封集合匹配、失败密文包保留和稳定失败提示；Renderer 不得读取私钥、DEK 或本地完整路径：`desktop/src/main/encryption/decryptionCoordinator.ts`、`desktop/src/main/encryption/rsaKeyStore.ts`
- [x] T098 [US2] 调整文件中心和路由显示逻辑，解密按钮只受文件可用状态控制，不再用 `can_decrypt` 或角色权限禁用本地解密尝试，并明确展示“可下载密文、客户端尝试解密”：`desktop/src/renderer/src/pages/FileCenterPage.tsx`、`desktop/src/renderer/src/components/encryption/FileTable.tsx`、`desktop/src/renderer/src/components/encryption/EncryptedFileDetail.tsx`、`desktop/src/renderer/src/main.tsx`
- [x] T099 [US2] 更新 API 契约、密文格式说明、README 和验收测试，明确服务端不保存私钥/原始 AES 密钥、不解密、不返回明文；完成关键注释和可读性检查：`specs/012-du-decryption/contracts/openapi.yaml`、`specs/010-do-encrypted-upload/contracts/ciphertext-format.md`、`backend/README.md`、`desktop/README.md`

---

## 依赖和执行顺序

### 阶段依赖

- 阶段 1 无依赖。
- 阶段 2 依赖阶段 1，且阻塞所有用户故事。
- 用户故事 1 是 MVP，必须先完成多接收者模型。
- 用户故事 2 依赖用户故事 1 的 protected DEK 一对多结构。
- 用户故事 3 依赖用户故事 2 的解密材料授权接口。
- 用户故事 4 依赖用户故事 1 的多接收者执行数据。
- 用户故事 5 依赖用户故事 2、3、4 提供的完整数据。
- 阶段 8 依赖阶段 2 的迁移能力和用户故事 1 的关键加密事务边界；T075-T082 测试先行。
- 最终阶段依赖选定用户故事和阶段 8 完成。

### 并行机会

- T002、T003、T004 可并行。
- T005、T008、T010 可并行。
- 每个用户故事中的测试任务可并行编写。
- T075-T082 的静态、规范化、仓储、状态机和故障注入测试可并行编写；T083-T090 必须按迁移、领域、仓储、服务、命令顺序收敛。
- 前端格式化工具和后端文件中心查询在阶段 7 前不可抢跑，但阶段 7 内 T057 与组件测试可并行。

### MVP 策略

1. 完成阶段 1 和阶段 2。
2. 完成用户故事 1，证明一个文件、一份密文、多份 protected DEK 成立。
3. 完成用户故事 2，证明授权边界正确。
4. 完成用户故事 3，证明本地解密安全闭环。
5. 再完成性能指标和页面重构。

## 格式校验

- 所有任务均使用 `- [ ] Txxx` 清单格式。
- 用户故事任务均带 `[USx]` 标签。
- 可并行任务使用 `[P]` 标记。
- 每个任务都包含明确文件路径或明确命令。

---

## 客户端加密收口阶段

本阶段覆盖旧任务完成后发现的产品语义收口：Electron 选择普通明文并在主进程/本地 Worker 完成加密，Go 后端只保存密文；密文读取不再按 RBAC 解密权限或接收者关系筛选。

- [x] T100 [US1] 确认并固化 Electron 本地 AES-256-GCM 分块加密、单份密文、多份 RSA-OAEP-SHA256 密钥信封上传协议：`desktop/src/main/encryption/coordinator.ts`、`desktop/src/main/encryption/cryptoWorkerProcess.ts`
- [x] T101 [US2] 增加客户端密文算法版本、Nonce 前缀、认证标签长度和 AAD 版本迁移及后端持久化字段：`backend/migrations/015_client_ciphertext_metadata.sql`、`backend/internal/domain/encrypted_file.go`、`backend/internal/service/encryption_service.go`
- [x] T102 [US2] 移除文件列表、详情、密文下载和信封读取中的解密授权字段与接收者筛选，统一按有效租户成员返回：`backend/internal/handler/router.go`、`backend/internal/repository/encryption_repository.go`、`backend/internal/service/encrypted_file_service.go`
- [x] T103 [US2] 将文件中心改为“全部密文/我的加密文件”，统一下载按钮，不展示分享关系、可解密、未授权状态：`desktop/src/renderer/src/pages/FileCenterPage.tsx`、`desktop/src/renderer/src/components/encryption/FileTable.tsx`、`desktop/src/renderer/src/components/encryption/EncryptedFileDetail.tsx`
- [x] T104 [US2] 在主进程实现流式保存原始 `.enc`、失败保留密文、错误私钥和 GCM 校验失败不输出明文：`desktop/src/main/encryption/decryptionCoordinator.ts`、`desktop/src/main/main.ts`、`desktop/src/preload/preload.ts`
- [x] T105 [US2] 增加主进程本地 RSA 私钥导入，使用 `safeStorage` 保存密文，Renderer 只获得公钥登记材料：`desktop/src/main/encryption/rsaKeyStore.ts`、`desktop/src/renderer/src/pages/MyRSAKeysPage.tsx`
- [x] T106 [US2] 完成关键注释和可读性检查，确认 GoDoc 前缀、业务语义注释、安全边界、IPC 私钥边界、无旧授权式 UI 文案：`backend/internal/`、`desktop/src/main/`、`desktop/src/renderer/src/`
- [x] T107 [US2] 执行 Go 全量测试、桌面端 Vitest、TypeScript 类型检查和 Electron 生产构建：`go test ./...`、`npm.cmd test -- --run`、`npm.cmd run typecheck`、`npm.cmd run build`
