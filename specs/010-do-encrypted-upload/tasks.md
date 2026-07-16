# 任务：DO 文件加密上传基础闭环与通用混合加密框架

**输入**：`specs/010-do-encrypted-upload/` 下的规格、计划、研究、数据模型和契约

**前置产物**：`plan.md`、`spec.md`、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试要求**：规格明确要求单元测试、集成测试、端到端测试、安全测试和性能验证，因此每个用户故事均包含测试先行任务。测试任务应先编写并确认在对应实现完成前失败。

**组织方式**：任务按用户故事组织，以支持独立实施和验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**：可与同阶段其他标记任务并行，前提是不修改同一文件且依赖已满足。
- **[US1]…[US5]**：对应 `spec.md` 的用户故事。
- 每个任务均包含明确文件路径。

---

## 阶段 1：准备工作（共享基础）

**目的**：准备配置、目录、迁移入口和测试骨架，不实现用户故事业务。

- [X] T001 在 `backend/internal/config/config.go` 增加密文根目录、暂存目录、1 GiB 上限、租户并发数和暂存过期时间配置，并为每个新增/修改函数补齐符合项目规范的中文前置注释
- [X] T002 [P] 在 `backend/.env.example` 和 `.env.example` 增加密文存储与加密任务配置示例，并统一桌面端与后端示例 API 端口
- [X] T003 [P] 在 `backend/internal/crypto/doc.go` 建立统一 Go `CryptoEngine` 包说明，明确远程 server 不接收文件明文或 DEK、本地 Crypto Worker 可在客户端设备内调用完整密码学能力
- [X] T004 [P] 在 `desktop/src/main/encryption/types.ts` 建立主进程文件句柄、执行描述、进度事件和脱敏错误的共享类型
- [X] T005 [P] 在 `desktop/src/main/encryption/cryptoWorkerProtocol.ts` 建立 Electron 与本地 Go Crypto Worker 的长度前缀控制协议，禁止响应返回明文 DEK、私钥和本地路径
- [X] T006 在 `backend/internal/migrations/sql_migrations.go` 注册 `011_encrypted_file_framework.sql`，保持既有迁移顺序、错误定位和中文函数注释

---

## 阶段 2：基础能力（阻塞所有用户故事）

**目的**：建立数据库、通用领域模型、存储、审计、算法目录、Repository 和桌面安全边界。

**关键门禁**：本阶段完成前不得开始任何用户故事实现。

### 基础测试

- [X] T007 [P] 在 `backend/internal/migrations/encryption_framework_test.go` 编写 011 迁移重复执行、唯一约束、权限种子、算法种子和后置校验测试
- [X] T008 [P] 在 `backend/internal/pkg/storage/encrypted_file_storage_test.go` 编写租户对象键、路径穿越拒绝、暂存提交、流式读取、删除幂等和哈希校验测试
- [X] T009 [P] 在 `backend/internal/crypto/catalog_test.go` 编写首期只启用 `RSAEngine`、未知算法拒绝、租户禁用、`CryptoEngine` 能力描述和未来 `TKN20Engine` 注册边界测试
- [X] T010 [P] 在 `backend/internal/repository/encryption_repository_test.go` 编写租户范围、幂等任务、行锁状态更新和跨租户不可见测试
- [X] T011 [P] 在 `backend/internal/repository/audit_repository_test.go` 编写审计白名单元数据、敏感字段拒绝和租户范围测试
- [X] T012 [P] 在 `desktop/src/main/encryption/ipcValidation.test.ts` 编写 IPC 发送方、UUID、租户、文件句柄、算法和上传域名校验测试

### 基础实现

- [X] T013 在 `backend/migrations/011_encrypted_file_framework.sql` 创建算法、租户算法、公钥、文件、密文对象、任务、执行、受保护密钥、RSA 绑定、Benchmark、孤儿对象和审计表，并种子化新权限与 RSA 首个算法
- [X] T014 在 `backend/internal/migrations/encryption_validation.go` 实现 011 迁移前后置校验，验证无重复版本、无跨租户外键、无缺失权限和无伪 CP-ABE 可用记录
- [X] T015 [P] 在 `backend/internal/domain/encrypted_file.go` 定义加密文件和密文对象实体及枚举，为每个实体字段说明来源、可空性、敏感性、租户和外部响应边界
- [X] T016 [P] 在 `backend/internal/domain/encryption_task.go` 定义任务、执行、状态转换和 Benchmark 实体，为每个函数/方法及字段补齐中文安全注释
- [X] T017 [P] 在 `backend/internal/domain/encryption_key.go` 定义算法、租户算法、RSA 公钥、通用受保护密钥和 RSA 绑定实体，确保通用实体没有固定 `rsa_*` 字段
- [X] T018 [P] 在 `backend/internal/domain/audit.go` 定义审计和孤儿对象实体、状态及允许的非敏感元数据结构
- [X] T019 在 `backend/internal/migrations/automigrate.go` 注册新增实体，同时保留显式 011 迁移作为约束和种子事实源
- [X] T020 在 `backend/internal/crypto/catalog.go` 实现算法能力目录和租户启用查询接口，首期只注册 `RSAEngine` 对应的 `RSA-OAEP-SHA256` 版本 1，不注册伪 CP-ABE
- [X] T021 在 `backend/internal/crypto/metadata_validator.go` 实现算法无关的授权快照、上下文摘要、受保护密钥格式和适配绑定校验接口
- [X] T022 在 `backend/internal/pkg/storage/storage.go` 拆分头像与密文存储能力，新增暂存、提交、打开下载流、删除和巡检接口且不破坏头像调用方
- [X] T023 在 `backend/internal/pkg/storage/encrypted_file_storage.go` 实现受控本地密文存储、不可预测租户对象键、临时 `.part`、服务端 SHA-256复核和原子重命名
- [X] T024 [P] 在 `backend/internal/repository/encryption_repository.go` 定义并实现文件、任务、执行、密文对象、受保护密钥、Benchmark 和完成事务的租户范围 Repository
- [X] T025 [P] 在 `backend/internal/repository/rsa_key_repository.go` 定义并实现 RSA 公钥版本分配、指纹去重、状态更新和同租户接收者查询
- [X] T026 [P] 在 `backend/internal/repository/audit_repository.go` 实现数据库审计记录、白名单元数据序列化和租户审计查询基础能力
- [X] T027 在 `backend/internal/service/audit_recorder.go` 用持久化审计实现替代本功能路径的 `NoopAuditRecorder`，明确安全关键事件写入失败的业务边界
- [X] T028 在 `backend/internal/pkg/response/errors.go` 增加算法、公钥、文件、任务、上传、状态冲突、哈希不一致、取消、重试和存储补偿的稳定业务错误码
- [X] T029 在 `backend/internal/migrations/rbac_validation.go` 扩展 RBAC 后置检查，确认 DO、DU 和租户管理员分别获得规划中的密钥与文件权限
- [X] T030 在 `desktop/src/main/encryption/ipcValidation.ts` 实现单用途 IPC 参数、发送方 frame 和 API 基地址允许列表校验
- [X] T031 在 `backend/internal/crypto/engine.go` 定义统一 `CryptoEngine`、算法无关 `DEKProtector`、引擎注册表和结果类型，禁止通用接口出现 RSA 固定字段
- [X] T032 在 `backend/internal/crypto/container.go` 实现 `GCPABE01` 规范化头、4 MiB 认证分块、nonce 前缀+块序号、分块 AAD、长度检查和容器摘要公共能力

**检查点**：迁移、实体、存储、审计、通用算法目录、Repository 和 IPC/适配器边界可独立测试；所有基础测试通过后进入用户故事。

---

## 阶段 3：用户故事 1——DO 使用 RSA+AES 加密上传文件（优先级：P1）🎯 MVP

**目标**：有效 DO 选择本地文件和同租户 RSA 接收者，完成客户端 AES-256-GCM 加密、RSA-OAEP-SHA-256 保护 DEK、密文上传和服务端完成事务。

**独立测试**：使用租户 A 的 DO、DU 和 DU 有效公钥完成单文件闭环；测试私钥能恢复 DEK 并还原文件；服务端、数据库、普通日志和 IPC 返回中不存在明文文件、明文 DEK或私钥。

### 用户故事 1 测试（先编写并确认失败）

- [X] T033 [P] [US1] 在 `backend/internal/service/rsa_key_service_test.go` 编写本人公钥登记、3072 位校验、指纹重算、版本递增、重复幂等、同租户接收者和失效公钥测试
- [X] T034 [P] [US1] 在 `backend/internal/service/encryption_service_test.go` 编写算法启用、DO 权限、不可变授权快照、跨租户公钥拒绝、任务幂等、用户/租户请求限流、3 个并发租约、第 4 个任务可重试拒绝、Redis 故障策略和完成事务测试
- [X] T035 [P] [US1] 在 `backend/internal/handler/encryption_handler_test.go` 按 `contracts/openapi.yaml` 编写算法、接收者、任务创建、上传和完成接口契约测试
- [X] T036 [P] [US1] 在 `backend/internal/handler/rsa_key_handler_test.go` 按 OpenAPI 编写本人公钥登记、列表、管理员禁用和敏感字段响应测试
- [X] T037 [P] [US1] 在 `backend/internal/pkg/storage/encrypted_file_upload_integration_test.go` 编写大于缓冲区的流式上传、Content-Length、客户端/服务端哈希不一致和暂存对象不可下载测试
- [X] T038 [P] [US1] 在 `backend/internal/crypto/container_test.go` 编写认证分块容器往返、块序号/总数/长度、AAD、篡改、截断、重排、追加记录、错误 tag 和 nonce 唯一性测试
- [X] T039 [P] [US1] 在 `backend/internal/crypto/rsa_engine_test.go` 编写 `RSAEngine` 的 RSA-OAEP-SHA-256 往返、错误私钥、错误 label、公钥指纹和 3072 位约束测试
- [X] T040 [P] [US1] 在 `desktop/src/main/encryption/rsaKeyStore.test.ts` 编写 Electron 33 同步 `safeStorage` 不可用、Linux `basic_text`/`unknown` 拒绝、账号/租户隔离、待登记重试和私钥文件无明文 PEM 测试
- [X] T041 [P] [US1] 在 `desktop/src/main/encryption/coordinator.test.ts` 编写文件句柄、工作线程、上传、完成、Token 不落盘、临时密文清理和错误脱敏集成测试
- [X] T042 [P] [US1] 在 `desktop/src/renderer/src/pages/DataEncryptionPage.test.tsx` 编写 DO 权限、文件展示、动态算法、RSA 接收者、公钥版本、确认和非法提交测试

### 用户故事 1 实现

- [X] T043 [US1] 在 `backend/internal/service/rsa_key_service.go` 实现公钥解析、SPKI 指纹重算、版本、本人登记、接收者查询和管理员状态变更业务规则，并显式审计公钥登记、选择、禁用和拒绝事件
- [X] T044 [US1] 在 `backend/internal/handler/rsa_key_handler.go` 实现本人公钥、接收者、公钥版本和管理员状态接口，响应不得包含任何私钥字段
- [X] T045 [US1] 在 `backend/internal/service/encryption_admission.go` 和 `backend/internal/service/encryption_service.go` 实现 Redis 用户/租户令牌桶、带 TTL 并发租约、Redis 故障安全策略、算法查询、幂等任务创建、DO/租户/接收者/公钥校验、授权快照冻结及任务创建/校验审计
- [X] T046 [US1] 在 `backend/internal/service/encryption_service.go` 实现密文上传前置校验、暂存对象登记、服务端哈希复核、上传失败清理，并写入 `SERVER_OBSERVED` 上传成功/失败审计
- [X] T047 [US1] 在 `backend/internal/service/encryption_service.go` 实现完成事务，校验上下文摘要、通用受保护密钥、RSA 专属绑定、Benchmark，区分 `CLIENT_REPORTED` 密码阶段与 `SERVER_OBSERVED` 完成事件，原子更新文件/对象/任务状态和安全关键审计
- [X] T048 [US1] 在 `backend/internal/handler/encryption_handler.go` 实现算法、任务创建、密文流上传和完成 Handler，限制请求大小且不把密码学堆栈返回前端
- [X] T049 [US1] 在 `backend/internal/handler/router.go` 注册 US1 路由及 `file.upload`、`crypto.key.self.manage`、`crypto.key.manage` 权限中间件
- [X] T050 [US1] 在 `backend/cmd/server/main.go` 装配算法目录、加密 Repository、RSA Key Service、加密 Service、密文存储和数据库审计依赖
- [X] T051 [P] [US1] 在 `desktop/src/main/encryption/rsaKeyStore.ts` 实现接收 Go Crypto Worker 生成的 PKCS#8 私钥、Electron 33 同步 `safeStorage.encryptString` 小体积保护、Linux 不安全后端拒绝、临时 Buffer 清理、SPKI 公钥登记材料和版本索引
- [X] T052 [P] [US1] 在 `desktop/src/main/encryption/fileSelection.ts`、`desktop/src/renderer/src/auth/AuthContext.tsx` 和 `desktop/src/renderer/src/api/authRuntime.ts` 实现原生单文件句柄及账号/租户切换开始时清空算法/接收者/公钥/文件/任务缓存、取消旧执行并释放主进程句柄
- [X] T053 [US1] 在 `backend/internal/crypto/rsa_engine.go` 实现首个 `RSAEngine`，显式使用 Go RSA-OAEP SHA-256、头摘要 label、公钥指纹校验和算法无关结果
- [X] T054 [US1] 在 `backend/internal/crypto/content_cipher.go` 和 `backend/cmd/crypto-worker/main.go` 实现随机 DEK/nonce 前缀、4 MiB 分块 AES-256-GCM、容器 SHA-256、`CryptoEngine` 调用、源文件变化检测、进度协议和 DEK Buffer 清理
- [X] T055 [P] [US1] 在 `desktop/src/main/encryption/tempCiphertext.ts` 实现随机 `.part` 路径、受限权限、成功/失败/取消删除和启动过期巡检
- [X] T056 [P] [US1] 在 `desktop/src/main/encryption/taskApiClient.ts` 实现受允许 API 基地址约束的任务创建、流式上传和完成请求，Authorization header 不得记录
- [X] T057 [US1] 在 `desktop/src/main/encryption/cryptoWorkerProcess.ts` 和 `desktop/src/main/encryption/coordinator.ts` 校验固定 Crypto Worker 路径与完整性、使用无 shell 子进程编排文件句柄/任务/上传/完成、映射脱敏审计错误并释放所有退出路径资源
- [X] T058 [US1] 在 `desktop/src/main/main.ts` 注册经过发送方验证的文件、RSA 密钥和加密执行 IPC Handler，并保持 `contextIsolation=true`、`nodeIntegration=false`
- [X] T059 [US1] 在 `desktop/src/preload/preload.ts` 暴露 `desktopEncryption` 单用途桥接方法，不直接暴露 `ipcRenderer` 或任意路径/URL 能力
- [X] T060 [P] [US1] 在 `desktop/src/renderer/src/api/encryption.ts` 实现算法、接收者、公钥、任务创建和完成结果的类型安全 API 封装，并提供账号/租户切换时的显式缓存清理入口
- [X] T061 [P] [US1] 在 `desktop/src/renderer/src/components/encryption/FileSelector.tsx` 实现单文件选择、展示、更换、移除和明确错误状态
- [X] T062 [P] [US1] 在 `desktop/src/renderer/src/components/encryption/AlgorithmAuthorizationForm.tsx` 实现后端驱动算法列表和 RSA 接收者/公钥版本表单，不写死通用框架为 RSA 表单
- [X] T063 [P] [US1] 在 `desktop/src/renderer/src/components/encryption/EncryptionConfirmation.tsx` 实现文件、算法、接收者、公钥版本和“仅上传密文”确认视图
- [X] T064 [US1] 在 `desktop/src/renderer/src/pages/DataEncryptionPage.tsx` 整合选择、校验、确认、任务创建和主进程启动，禁止在 React 状态保存 Token 以外的密钥材料
- [X] T065 [US1] 在 `desktop/src/renderer/src/main.tsx` 注册受 `file.upload` 保护的数据加密路由，并在 `desktop/src/renderer/src/components/AppLayout.tsx` 增加唯一菜单入口
- [X] T066 [US1] 在 `desktop/src/renderer/src/vite-env.d.ts` 声明 `desktopEncryption` 的严格 Window 类型，确保渲染层无法调用未声明的 IPC
- [X] T067 [US1] 在 `desktop/src/main/encryption/encryption.e2e.test.ts` 构建并启动 `backend/cmd/crypto-worker`，完成“DO 登录到任务完成”的本地 Go 加密与后端测试服务端到端验证，并断言测试私钥可还原原文件

**检查点**：US1 可独立演示 RSA+AES 主闭环，且通用协调器只依赖 `DekProtectionAdapter`。

---

## 阶段 4：用户故事 2——DO 查看真实加密进度（优先级：P1）

**目标**：DO 能查看合法阶段、已处理字节和取消可用性，大文件处理期间界面保持响应且无随机伪进度。

**独立测试**：执行一个可观察时长的文件加密，阶段按合法顺序变化，字节数不倒退，无法精确计算时只显示阶段，React 界面持续可交互。

### 用户故事 2 测试（先编写并确认失败）

- [X] T068 [P] [US2] 在 `backend/internal/service/encryption_progress_test.go` 编写合法阶段、跳阶段拒绝、字节倒退、终态更新、跨租户和取消可用性测试
- [X] T069 [P] [US2] 在 `backend/internal/handler/encryption_progress_handler_test.go` 编写进度接口契约、节流后的重复请求和敏感字段响应测试
- [X] T070 [P] [US2] 在 `desktop/src/main/encryption/progressBridge.test.ts` 编写工作线程进度合并、脱敏、取消订阅和多执行隔离测试
- [X] T071 [P] [US2] 在 `desktop/src/renderer/src/components/encryption/EncryptionProgress.test.tsx` 编写阶段文案、字节进度、无伪百分比、失败和取消按钮状态测试

### 用户故事 2 实现

- [X] T072 [US2] 在 `backend/internal/service/encryption_service.go` 增加基于行锁和合法状态图的阶段/字节进度更新，标记 Crypto Worker 阶段为 `CLIENT_REPORTED` 审计，终态后拒绝继续推进
- [X] T073 [US2] 在 `backend/internal/handler/encryption_handler.go` 增加进度上报和任务状态查询 Handler，并在 `backend/internal/handler/router.go` 注册对应路由
- [X] T074 [US2] 在 `backend/cmd/crypto-worker/main.go` 按真实分块读取字节和阶段发送长度前缀进度帧，禁止基于计时器生成随机百分比
- [X] T075 [US2] 在 `desktop/src/main/encryption/cryptoWorkerProcess.ts` 和 `desktop/src/main/encryption/coordinator.ts` 实现 Go 子进程进度解析、节流、服务端上报、窗口事件分发和执行间隔离
- [X] T076 [US2] 在 `desktop/src/preload/preload.ts` 增加安全进度订阅与移除监听器能力，并在 `desktop/src/renderer/src/vite-env.d.ts` 同步类型
- [X] T077 [US2] 在 `desktop/src/renderer/src/components/encryption/EncryptionProgress.tsx` 实现阶段时间线、已处理字节、可计算百分比和取消可用性展示
- [X] T078 [US2] 在 `desktop/src/renderer/src/pages/DataEncryptionPage.tsx` 接入进度订阅、页面卸载清理和错误恢复，并校验事件 `accountId`/`tenantId` 与当前 `AuthContext` 一致后再更新页面

**检查点**：US2 可在 US1 执行器上独立验证真实进度与 UI 响应，不改变密码学结果。

---

## 阶段 5：用户故事 3——DO 管理自己的加密文件（优先级：P1）

**目标**：DO 只查看当前租户自己创建的文件和任务，并鉴权下载已完成密文；失败、取消和半成品不可下载。

**独立测试**：准备完成、失败和取消记录，验证列表/详情只有当前 DO 数据，完成密文下载摘要一致，其他状态和其他租户/所有者下载均被拒绝。

### 用户故事 3 测试（先编写并确认失败）

- [X] T079 [P] [US3] 在 `backend/internal/repository/encrypted_file_query_test.go` 编写租户+所有者分页、状态筛选、详情脱敏和稳定排序测试
- [X] T080 [P] [US3] 在 `backend/internal/service/encrypted_file_service_test.go` 编写列表、详情、完成下载、非完成拒绝、对象缺失和哈希记录测试
- [X] T081 [P] [US3] 在 `backend/internal/handler/encrypted_file_handler_test.go` 按 OpenAPI 编写列表、详情、流式下载、Content-Disposition 和跨租户 404 测试
- [X] T082 [P] [US3] 在 `desktop/src/renderer/src/pages/EncryptedFilesPage.test.tsx` 编写分页、状态、算法、接收者、详情和下载可用性测试

### 用户故事 3 实现

- [X] T083 [US3] 在 `backend/internal/repository/encryption_repository.go` 实现按可信租户与 owner 查询文件分页、详情和可用密文对象，禁止仅按外部 UUID 查询
- [X] T084 [US3] 在 `backend/internal/service/encrypted_file_service.go` 实现 DO 自有列表、脱敏详情和可用状态下载授权，管理员角色不得自动越过所有者边界，并写入下载成功、拒绝和对象异常审计
- [X] T085 [US3] 在 `backend/internal/handler/encrypted_file_handler.go` 实现分页列表、详情和流式密文下载，响应隐藏对象键与完整受保护 DEK
- [X] T086 [US3] 在 `backend/internal/handler/router.go` 注册 `file.read` 保护的文件查询与下载路由
- [X] T087 [P] [US3] 在 `desktop/src/renderer/src/api/encryption.ts` 增加加密文件分页、详情和密文下载 API，并校验下载摘要响应头
- [X] T088 [P] [US3] 在 `desktop/src/renderer/src/components/encryption/EncryptedFileDetail.tsx` 实现脱敏详情、算法/公钥版本摘要和状态操作区
- [X] T089 [US3] 在 `desktop/src/renderer/src/pages/EncryptedFilesPage.tsx` 实现 DO 自有文件列表、分页、筛选、详情和仅完成项下载
- [X] T090 [US3] 在 `desktop/src/renderer/src/main.tsx` 和 `desktop/src/renderer/src/components/AppLayout.tsx` 注册 `file.read` 保护的“我的加密文件”路由与菜单

**检查点**：US3 可只依赖基础数据和完成记录验收，不需要重试或未来算法实现。

---

## 阶段 6：用户故事 4——DO 安全重试失败任务（优先级：P1）

**目标**：上传、保存、退出等失败产生可追踪终态；DO 可对可恢复错误创建新执行，半成品和孤儿对象不可下载且可清理。

**独立测试**：注入上传失败、完成事务失败、对象删除失败和 Electron 异常退出，验证旧执行保留、新执行重新加密、完成结果唯一、孤儿可清理且无 DEK 恢复文件。

### 用户故事 4 测试（先编写并确认失败）

- [X] T091 [P] [US4] 在 `backend/internal/service/encryption_state_machine_test.go` 编写失败分类、取消竞争、重试序号、不可变授权、重复完成和主动新任务区分测试
- [X] T092 [P] [US4] 在 `backend/internal/service/encryption_compensation_test.go` 编写上传失败、数据库回滚、对象删除失败、孤儿登记和清理重试测试
- [X] T093 [P] [US4] 在 `backend/internal/handler/encryption_retry_handler_test.go` 编写取消、重试、重复幂等、不可重试错误和跨租户拒绝契约测试
- [X] T094 [P] [US4] 在 `desktop/src/main/encryption/recovery.test.ts` 编写异常退出、过期 `.part` 巡检、旧文件句柄失效、无 DEK 恢复数据和全量重试测试
- [X] T095 [P] [US4] 在 `desktop/src/renderer/src/components/encryption/RetryAction.test.tsx` 编写可重试提示、重新选择文件、取消和重复点击幂等测试

### 用户故事 4 实现

- [X] T096 [US4] 在 `backend/internal/service/encryption_service.go` 实现错误分类、`retryable`、取消状态竞争、同任务新执行、重复完成幂等返回、并发租约终态释放及失败/取消/重试审计
- [X] T097 [US4] 在 `backend/internal/service/orphan_cleanup_service.go` 实现失败对象登记、指数退避清理、并发租约过期巡检、租户范围、稳定脱敏错误和清理结果审计
- [X] T098 [US4] 在 `backend/internal/repository/encryption_repository.go` 实现孤儿对象领取、`SKIP LOCKED` 批处理、清理结果和过期暂存对象查询
- [X] T099 [US4] 在 `backend/cmd/cleanup/main.go` 装配可重复运行的孤儿/过期暂存清理命令，提供批量上限和安全退出
- [X] T100 [US4] 在 `backend/internal/handler/encryption_handler.go` 实现取消与重试 Handler，并在 `backend/internal/handler/router.go` 注册 `file.manage` 路由
- [X] T101 [US4] 在 `desktop/src/main/encryption/cryptoWorkerProcess.ts` 和 `desktop/src/main/encryption/coordinator.ts` 实现 AbortSignal、Go Crypto Worker 终止、上传中止、服务端取消和所有终态临时文件清理
- [X] T102 [US4] 在 `desktop/src/main/encryption/recovery.ts` 实现启动巡检、非敏感执行索引恢复和未完成任务中断上报，不自动续传或恢复 DEK
- [X] T103 [US4] 在 `desktop/src/main/main.ts` 注册取消、清理和启动恢复流程，并保证应用退出时尽力释放文件句柄与工作线程
- [X] T104 [P] [US4] 在 `desktop/src/renderer/src/components/encryption/RetryAction.tsx` 实现按错误类型提示重新选择文件或直接重试，防止重复点击
- [X] T105 [US4] 在 `desktop/src/renderer/src/pages/DataEncryptionPage.tsx` 和 `desktop/src/renderer/src/pages/EncryptedFilesPage.tsx` 接入取消、失败详情和重试后新执行状态

**检查点**：US4 的所有故障注入场景均不产生可下载半成品，且重试不会覆盖旧有效密文。

---

## 阶段 7：用户故事 5——后续接入新的 DEK 保护算法（优先级：P2）

**目标**：用非产品测试适配器证明新增算法只提供授权校验和 DEK 保护结果，不复制 AES、上传、任务、列表、补偿或审计。

**独立测试**：注册仅测试环境可见的适配器，运行同一协调器和容器流程；产品算法列表仍只有 RSA，通用数据库实体无 RSA 固定字段，测试适配器不冒充 CP-ABE。

### 用户故事 5 测试（先编写并确认失败）

- [X] T106 [P] [US5] 在 `backend/internal/crypto/engine_contract_test.go` 编写 Go `CryptoEngine`/`DEKProtector` 注册、授权校验、输出格式、未知算法拒绝和非产品测试引擎复用协调器的契约测试
- [X] T107 [P] [US5] 在 `backend/internal/crypto/metadata_validator_contract_test.go` 编写算法无关授权/受保护密钥校验和 RSA 专属绑定隔离测试
- [X] T108 [P] [US5] 在 `backend/internal/domain/encryption_algorithm_independence_test.go` 通过反射/字段白名单断言通用任务、文件和受保护密钥实体不存在 `rsa_*` 字段
- [X] T109 [P] [US5] 在 `desktop/src/renderer/src/components/encryption/AlgorithmAuthorizationForm.test.tsx` 编写按 `authorization_type` 选择表单、未知类型安全禁用和不永久写死 RSA 的测试

### 用户故事 5 实现

- [X] T110 [US5] 在 `backend/internal/crypto/engine.go` 完善 Go 引擎注册、能力匹配、授权规范化和通用错误映射，非产品测试引擎仅在测试构建注册
- [X] T111 [US5] 在 `backend/internal/crypto/metadata_validator.go` 完善按算法 code/version 分派专属校验器的机制，通用完成事务只消费统一验证结果
- [X] T112 [US5] 在 `backend/internal/service/encryption_service.go` 移除任何 RSA 固定分支并通过算法校验器返回的通用绑定写入计划选择的专属 Repository
- [X] T113 [US5] 在 `desktop/src/renderer/src/components/encryption/AlgorithmAuthorizationForm.tsx` 建立授权表单注册表，RSA 作为首个 `RSA_RECIPIENT` 表单实现，未知类型只显示不可用原因
- [X] T114 [US5] 在 `specs/010-do-encrypted-upload/contracts/desktop-ipc.md` 和 `specs/010-do-encrypted-upload/data-model.md` 复核新增 Go `CryptoEngine` 接入步骤，明确下一期优先评估真实 Go TKN20，只有形成不适用结论后才可选择真实 Go BSW07 或其他方案

**检查点**：US5 证明 RSA 是首个 `CryptoEngine` 实现而非框架依赖，且没有实现或展示伪 CP-ABE。

---

## 阶段 8：收尾与横切验证

**目的**：完成跨故事安全、性能、审计、文档和项目宪章检查。

- [X] T115 [P] 在 `backend/internal/handler/encryption_security_test.go` 增加伪造任务/文件/公钥 ID、路径穿越、重放完成、密文篡改、跨租户和超大请求安全测试
- [X] T116 [P] 在 `desktop/src/main/encryption/security.test.ts` 增加 IPC sender 伪造、任意 URL、路径泄漏、Token 日志、私钥明文和账号/租户切换缓存安全测试
- [X] T117 [P] 在 `backend/internal/service/audit_encryption_test.go` 覆盖创建、校验、AES 完成、DEK 保护、上传、完成、失败、取消、重试、下载、清理失败和越权审计事件
- [X] T118 [P] 在 `backend/internal/crypto/benchmark_test.go` 和 `desktop/src/main/encryption/benchmark.test.ts` 增加 1 GiB 分块 AES、RSA DEK 保护、Go Worker 内存峰值、取消响应、3 个并发任务和第 4 个任务可重试拒绝基准记录
- [X] T119 在 `backend/internal/service/encryption_service.go` 和 `backend/internal/domain/encryption_task.go` 复核 Benchmark 口径，确保 AES、DEK 保护、上传和密文大小分别保存且不生成误导结论
- [X] T120 在 `backend/internal/handler/router.go` 和 `backend/cmd/server/main.go` 复核密文目录未注册静态路由、所有加密路由具备认证/租户/permission code 中间件和非空依赖
- [X] T121 在 `desktop/src/main/main.ts`、`desktop/src/main/encryption/cryptoWorkerProcess.ts` 和 `desktop/src/preload/preload.ts` 复核 Electron 安全配置、IPC sender 校验、固定 Go Worker 完整性、无 shell 启动、单用途桥接、导航限制和敏感参数边界
- [X] T122 [P] 在 `backend/README.md` 增加本期配置、迁移、清理命令、工程演示安全声明和分离 Benchmark 指标说明
- [X] T123 [P] 在 `desktop/README.md` 创建桌面端文件选择、本机 RSA 私钥安全存储、失败恢复和不提供解密页面的中文说明
- [X] T124 在 `specs/010-do-encrypted-upload/quickstart.md` 逐项执行并记录 Go 测试、TypeScript 类型检查、Vitest、构建、端到端、失败补偿、跨租户、日志扫描和性能结果
- [X] T125 对 `backend/internal/crypto/`、`backend/internal/domain/`、`backend/internal/handler/`、`backend/internal/service/`、`backend/internal/repository/`、`backend/internal/middleware/`、`backend/internal/pkg/storage/` 和 `backend/cmd/` 完成关键注释和可读性检查：所有函数/方法有前置中文注释，导出标识符符合 GoDoc 前缀，实体字段与核心模块解释业务语义、副作用、幂等、租户/鉴权/密码学安全边界，并清理无意义逐行注释
- [X] T126 在 `backend/` 执行 `gofmt`、`go vet ./...`、`go test ./...`，修复失败并将最终命令与结果记录到 `specs/010-do-encrypted-upload/quickstart.md`
- [X] T127 在 `desktop/` 执行 `npm run typecheck`、`npm test`、`npm run build`，修复失败并将最终命令与结果记录到 `specs/010-do-encrypted-upload/quickstart.md`
- [X] T128 在 `specs/010-do-encrypted-upload/checklists/requirements.md` 复核实现与 FR/CRYPTO/RSA/STO/DATA/TASK/TEN/AUTH/SEC/REC/IDEM/AUD/PERF/BENCH/AC/SC 编号的覆盖关系并记录任何偏离

---

## 依赖与执行顺序

### 阶段依赖

- **阶段 1（准备）**：无依赖，可立即开始。
- **阶段 2（基础）**：依赖阶段 1，阻塞全部用户故事。
- **阶段 3（US1）**：依赖阶段 2，是可演示 MVP。
- **阶段 4（US2）**：依赖阶段 2 的任务状态模型和 US1 的工作线程/协调器；可在 US1 核心接口稳定后并行开发 UI 与后端进度能力。
- **阶段 5（US3）**：依赖阶段 2；其查询能力可并行开发，但完整独立验收需要一个 US1 完成记录。
- **阶段 6（US4）**：依赖阶段 2 的状态机和存储补偿；完整验收需要 US1 上传/完成链路。
- **阶段 7（US5）**：依赖阶段 2 的通用算法边界；契约测试可与 US1 并行，最终复核应在 US1 完成后执行。
- **阶段 8（收尾）**：依赖所有纳入本期交付的用户故事完成。

### 用户故事依赖图

```text
准备 → 基础 ─┬→ US1 RSA+AES 主闭环 ─┬→ US2 真实进度
             │                       ├→ US3 文件管理
             │                       └→ US4 重试补偿
             └→ US5 通用适配器契约 ─────→ 最终算法独立性复核

US1 + US2 + US3 + US4 + US5 → 收尾验证
```

### 故事内执行规则

- 测试任务必须先编写并确认失败，再实现对应代码。
- 实体与 Repository 先于 Service，Service 先于 Handler 和路由。
- 密文格式和适配器先于工作线程，工作线程先于协调器和 UI 集成。
- 每个检查点必须独立通过后再将该故事视为完成。

## 并行机会

### US1 并行示例

- 并行编写 T033–T042 的后端、桌面密码学、密钥存储和页面测试。
- 基础接口稳定后，并行执行 T051（密钥存储）、T052（文件选择）、T055（临时密文）、T056（API Client）和 T060–T063（渲染组件）。
- T053 依赖 T031；T054 依赖 T032 和 T053；T057 依赖 T054–T056。

### US2 并行示例

- T068–T071 可分别在 Service、Handler、主进程桥接和 React 组件中并行编写。
- T072–T073 后端链与 T074–T077 桌面链可并行，T078 最后集成。

### US3 并行示例

- T079–T082 可并行编写。
- T083–T086 后端查询下载链与 T087–T088 前端 API/详情组件可并行，T089–T090 最后集成。

### US4 并行示例

- T091–T095 可并行编写故障、补偿、恢复和 UI 测试。
- T097–T099 孤儿清理链与 T101–T104 客户端取消/恢复链可并行，T105 最后集成。

### US5 并行示例

- T106–T109 可并行验证桌面适配器、后端校验器、领域模型和动态表单。
- T110–T111 可并行完善客户端与后端注册机制，T112–T114 在两者稳定后执行。

## 实施策略

### MVP 优先（只完成 US1）

1. 完成阶段 1。
2. 完成阶段 2，并通过全部基础门禁。
3. 完成阶段 3 的 US1。
4. 停止扩展，按 US1 独立测试确认 RSA+AES 主闭环、密钥边界和服务端无明文。
5. MVP 稳定后再进入进度、文件管理和失败补偿增强。

### 增量交付

1. **基础 + US1**：得到真实 RSA+AES 加密上传 MVP。
2. **加入 US2**：用户可理解长任务进度且界面不卡死。
3. **加入 US3**：形成文件记录查询和密文取回闭环。
4. **加入 US4**：达到故障可恢复、半成品可清理的一致性目标。
5. **加入 US5**：证明框架不依赖 RSA，为下一期优先接入真实 Go TKN20、或在研究否决后接入其他真实 Go CP-ABE 做准备。
6. **阶段 8**：统一完成安全、性能、审计、注释和快速验证。

## 备注

- `[P]` 只表示文件和直接依赖允许并行；若前置基础任务未完成，仍不得提前执行。
- 所有 Go 新增或修改函数/方法必须有前置中文注释；导出标识符必须以名称开头并符合 GoDoc。
- 密码学、文件上传、Token、权限、租户、事务、并发、缓存和审计代码必须解释设计原因与安全边界。
- 每个任务或逻辑任务组完成后再提交，提交信息必须使用符合项目规范的简体中文 Conventional Commit + gitmoji。
- 不得在本期任务中实现或展示模拟 CP-ABE、访问树秘密共享、拉格朗日插值或 RSA 解密页面；下一期算法选择必须遵守 TKN20 优先评估门禁。
