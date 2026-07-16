# 实施计划：DO 文件加密上传基础闭环与通用混合加密框架

**功能目录**：`010-do-encrypted-upload` | **日期**：2026-07-11 | **规格**：[spec.md](./spec.md)

**输入**：`specs/010-do-encrypted-upload/spec.md`

## 摘要

本期建立由 Electron 负责本地文件选择与受控编排、本地 Go Crypto Worker 执行文件内容加密与 DEK 保护、远程 Go 服务端负责可信授权、任务编排、密文存储和审计的单文件闭环。Go Crypto 模块通过统一 `CryptoEngine` 提供算法能力，以 `RSAEngine` 作为首个实现完成 AES-256-GCM + RSA-OAEP-SHA-256；RSA+AES 是首个可运行方案，但通用文件流程、任务数据模型和上传协议均不依赖 RSA 接收者、公钥字段或 RSA 密文格式。

服务端新增算法目录、RSA 公钥版本、加密文件、密文对象、加密任务与执行、通用受保护密钥、RSA 适配绑定、孤儿对象和审计记录。Electron 通过受限 IPC 选择文件、管理本机 RSA 私钥保护、启动固定路径的本地 Go Crypto Worker、上传密文并报告真实进度；Crypto Worker 在同一进程内生成 DEK、完成 AES 和 RSA，且不返回明文 DEK。任务完成采用“上传暂存对象 → 数据库事务提交元数据 → 对象转为可下载”的顺序；失败对象删除或进入孤儿清理队列。

## 技术上下文

**语言/版本**：Go 1.23；TypeScript 5.7.2；Electron 33.4.11（锁文件解析版本）；React 19

**主要依赖**：Gin 1.10.1、Gorm 1.25.12、MySQL 驱动 1.5.7、go-redis 9.7.0；Go 标准库 `crypto/aes`、`crypto/cipher`、`crypto/rsa`、`crypto/sha256`、`crypto/rand`；Electron `ipcMain`/`contextBridge`/`safeStorage`；Node.js 内置 `child_process`、`fs`、`stream`

**存储**：MySQL 保存业务元数据与审计；Redis 保留现有会话能力并用于短期限流/并发计数；首期密文使用后端受控本地存储目录，必须通过存储接口访问，不暴露静态 URL；Electron 临时目录只短期保存密文 `.part` 文件

**测试**：Go `testing`、Gin `httptest`、Gorm 集成测试；Vitest；Node/Electron 主进程模块单元测试；端到端验收脚本和故障注入测试

**目标平台**：Windows 优先的 Electron 桌面端；Go HTTP 服务运行于项目现有开发环境；设计保持 macOS/Linux 文件和安全存储语义可扩展

**项目类型**：Electron 桌面应用 + Go Web 服务

**性能目标**：1 GiB 文件加密内存峰值不超过 256 MiB；加密时 95% 非加密交互在 200 ms 内反馈；可取消阶段 95% 在 2 秒内停止处理新数据；同租户至少 3 个并发任务

**约束**：明文文件原则上不上传；明文 DEK 不落库、不落盘、不进 IPC 返回值和日志；单文件上限 1 GiB；首期不支持断点续传、暂停/恢复、多接收者或解密页面；RSA 私钥只能存于客户端系统安全存储保护的数据中

**规模/范围**：新增 1 个桌面加密工作台、1 套主进程加密协调模块、约 15 组租户 API 路径、12 个核心数据实体、1 个通用 DEK 保护适配接口及 RSA 首个实现

## 宪章检查

*门禁：Phase 0 前检查通过；Phase 1 设计完成后再次检查。*

- **混合加密边界**：通过。文件内容只由 AES-256-GCM 加密；RSA-OAEP 只保护 32 字节 DEK，不直接处理文件内容。
- **真实 CP-ABE 实现**：通过。本期明确不实现 CP-ABE，也不加入模拟算法。下一期接入时必须选用真实 Go 密码学库；本期测试适配器只能验证接口，不得作为算法功能展示。
- **模块边界**：通过。`backend/internal/crypto` 定义统一 `CryptoEngine`、内容加密能力和首个 `RSAEngine`；`backend/cmd/crypto-worker` 在客户端设备本地调用该模块且不向远程服务端暴露 DEK。Electron 只负责受控进程编排；File、Task、Repository、Benchmark、Audit 各自负责业务边界，Handler 和 Electron 不含密码算法。
- **算法对比口径**：通过。分别记录 AES 文件加密、RSA DEK 保护、密文上传、密文大小和内存峰值；不得合并为“RSA 加密文件耗时”。
- **可解释性**：通过。本期展示任务阶段、算法版本、接收者、公钥版本、失败代码和密文可用性；访问策略、属性匹配、访问树和 LSSS 延至下一期，不在本期伪造。
- **中文文档**：通过。`plan.md`、`research.md`、`data-model.md`、`quickstart.md` 和 `contracts/` 说明均使用简体中文，代码标识符和协议字段除外。
- **Go 注释策略**：通过。后续所有 Go 函数和方法均添加前置中文注释；导出标识符使用 GoDoc 前缀；实体字段解释来源、敏感性、租户/权限参与方式；Handler、Service、Repository、Middleware、Crypto、Storage、Benchmark、Audit 注释说明事务、副作用、幂等和安全边界。
- **关键注释和可读性检查**：通过。后续 `tasks.md` 必须包含独立检查任务，验证函数/方法前置注释、GoDoc 前缀、实体字段、核心业务语义、租户/鉴权/密码学边界及无意义注释清理。

## 关键架构决定

### 1. RSA 是首个适配器，不是框架分支

Go Crypto 模块定义统一 `CryptoEngine`，其中 DEK 保护部分暴露算法无关的 `DEKProtector`：输入为仅存在于本地 Crypto Worker 内存中的 DEK、算法能力、规范化授权配置和绑定上下文；输出为通用受保护密钥、格式版本、上下文摘要和引擎专属绑定数据。首个 `RSAEngine` 只在自身内部读取接收用户、公钥版本和公钥材料；未来 `TKN20Engine` 复用相同接口。

Electron 通用协调器只处理文件选择、Crypto Worker 生命周期、临时密文、上传、任务状态和补偿；Go Crypto Worker 内的通用协调器处理文件校验、DEK 生命周期、AES 流和 `CryptoEngine` 调用。后端通用表只保存算法 code/version、授权快照 JSON、受保护密钥及上下文摘要；RSA 公钥关系进入独立的 RSA 绑定表。禁止在通用任务、文件或密文表新增 `rsa_*` 固定字段。

### 2. 客户端加密与进程隔离

文件选择由 Electron 主进程原生对话框完成，渲染进程只获得随机文件句柄和脱敏元数据，不获得完整路径。主进程验证 IPC 发送方和参数，并使用 `child_process.spawn` 直接启动随应用分发、固定路径且经过完整性校验的本地 Go Crypto Worker，不经过 shell。文件路径只通过受控本地进程协议传递给 Crypto Worker；随机数生成、AES、RSA、哈希和 DEK 清理全部在 Go 进程内完成。渲染进程只能调用单用途桥接方法并接收脱敏进度事件。

主进程加密协调器负责向既有 API 基地址发起任务、上传和完成请求；禁止渲染层传入任意上传域名。Access Token 只在一次受控执行所需时间内进入主进程内存，不写日志和磁盘。

### 3. 密文容器与完整性

采用版本化 `GCPABE01` 分块容器：固定魔数、格式版本、头长度、规范化 JSON 头，以及按顺序保存的认证分块。每次加密生成 32 字节 DEK和 8 字节随机 nonce 前缀；每个分块使用“8 字节随机前缀 + 4 字节大端块序号”组成唯一 12 字节 nonce，并由 Go `cipher.AEAD.Seal` 产生独立 16 字节认证标签。默认分块大小为 4 MiB，头部固定记录明文总长度、分块大小和总块数。

每个分块的 AAD 由规范化头摘要、块序号、总块数和该块明文长度组成，至少间接绑定租户 ID、所有者 ID、任务 ID、执行 ID、原始大小、内容算法、DEK 保护算法及版本、授权快照摘要。读取方必须从 0 到 `chunk_count-1` 连续验证块序号和预期长度，因此可检测重排、截断、重复和缺失。RSA-OAEP 使用规范化头的 SHA-256 摘要作为 OAEP label，从而同时绑定密文、受保护 DEK、接收者和公钥版本。容器整体另计算 SHA-256 用于上传校验和存储巡检。

### 4. 任务、幂等与补偿

创建任务使用 `(tenant_id, owner_id, idempotency_key)` 唯一约束；用户主动改变接收者、公钥或算法时必须使用新的幂等键。每次重试在同一任务下创建新的执行记录，并重新生成 DEK、nonce 和临时密文，不续传旧加密输出。

状态转换在 MySQL 事务中锁定任务和当前执行行。上传先写同租户暂存对象并核对长度/哈希，数据库完成事务成功后才标为可下载。若数据库提交失败，立即删除暂存对象；删除失败则登记孤儿对象。重复完成请求返回既有结果，不重复插入密文或受保护密钥。

### 5. RSA 公钥和客户端私钥

本期补齐本地 Go Crypto Worker 生成 RSA 3072 位密钥对、服务端登记 SPKI PEM 公钥和历史版本、用户查看自己的公钥、租户管理员禁用公钥、接收者查询有效版本。Crypto Worker 将 PKCS#8 PEM 私钥交给 Electron 主进程的单用途密钥保存调用，主进程使用 Electron 33 已支持的同步 `safeStorage.encryptString` 立即保护后写入 `userData` 下的租户/账号隔离文件，并清理 IPC Buffer；该同步调用只允许处理小体积密钥且不得放入高频或长耗时循环。安全存储不可用，或 Linux `getSelectedStorageBackend()` 返回 `basic_text`/`unknown` 时，禁止生成和保存私钥。

### 6. 请求频率与并发准入

服务端在创建任务前调用 `EncryptionAdmissionController`。请求频率使用 Redis 原子令牌桶按用户和租户双层限制；并发任务使用带 TTL 的租约计数，在任务进入终态时释放，并由后台巡检修复异常退出造成的过期租约。Redis 不作为业务事实源：Redis 不可用时，创建任务必须采用安全失败，或在显式配置允许时回退到数据库统计非终态执行数；任何回退都必须记录审计。第 4 个超限任务返回稳定、可重试的业务错误，不创建文件、任务或 DEK。

### 7. 账号与租户切换清理

`AuthContext` 在账号或租户切换开始时先进入未就绪状态，清空渲染层算法、接收者、公钥、文件和任务缓存，并通过单用途 IPC 通知主进程取消旧执行、释放旧文件句柄和清理旧上下文临时密文。只有新账号和新租户授权上下文重新就绪后才允许选择文件或创建任务；任何带旧 `accountId`/`tenantId` 的进度事件均被丢弃。

### 8. 审计调用与信任等级

Service 在任务创建/校验、授权快照、公钥选择、上传、完成、失败、取消、重试、下载和清理补偿处显式调用 `AuditRecorder`。来自本地 Crypto Worker 的 AES 完成、DEK 保护和 Benchmark 事件标记为 `CLIENT_REPORTED`，服务端观察到的上传、事务、鉴权和存储事件标记为 `SERVER_OBSERVED`，不得混淆两者的可信度。安全关键审计写入与业务事务采用同事务或可恢复 outbox；写入失败时业务失败或进入可恢复状态，不静默丢失。

服务端只接收公钥，校验 RSA 类型、位数、指纹、重复和成员关系。公钥禁用只阻止新任务；历史文件继续引用当时版本，不静默切换到新公钥。本期用自动化测试的私钥路径验证解封装，不提供 DU 解密 UI。

## 项目结构

### 本功能文档

```text
specs/010-do-encrypted-upload/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── openapi.yaml
│   ├── desktop-ipc.md
│   └── ciphertext-format.md
└── tasks.md                  # 由后续 /speckit-tasks 生成
```

### 源代码规划

```text
backend/
├── migrations/
│   └── 011_encrypted_file_framework.sql
├── internal/
│   ├── crypto/
│   │   ├── catalog.go
│   │   ├── engine.go
│   │   ├── content_cipher.go
│   │   ├── rsa_engine.go
│   │   ├── container.go
│   │   └── metadata_validator.go
│   ├── domain/
│   │   ├── encrypted_file.go
│   │   ├── encryption_task.go
│   │   ├── encryption_key.go
│   │   └── audit.go
│   ├── handler/
│   │   ├── encryption_handler.go
│   │   ├── rsa_key_handler.go
│   │   └── router.go
│   ├── repository/
│   │   ├── encryption_repository.go
│   │   ├── rsa_key_repository.go
│   │   └── audit_repository.go
│   ├── service/
│   │   ├── encryption_service.go
│   │   ├── rsa_key_service.go
│   │   ├── audit_recorder.go
│   │   └── orphan_cleanup_service.go
│   └── pkg/storage/
│       ├── storage.go
│       └── encrypted_file_storage.go
└── cmd/
    ├── server/main.go
    ├── crypto-worker/main.go
    └── cleanup/main.go

desktop/src/
├── main/
│   ├── encryption/
│   │   ├── coordinator.ts
│   │   ├── fileSelection.ts
│   │   ├── taskApiClient.ts
│   │   ├── tempCiphertext.ts
│   │   ├── rsaKeyStore.ts
│   │   └── cryptoWorkerProcess.ts
│   └── main.ts
├── preload/preload.ts
└── renderer/src/
    ├── api/encryption.ts
    ├── pages/DataEncryptionPage.tsx
    ├── pages/EncryptedFilesPage.tsx
    ├── components/encryption/
    └── auth/permissions.ts
```

**结构决定**：沿用现有 Go Handler → Service → Repository 分层和 Electron 主进程/预加载/渲染进程分层。所有密码学操作进入 `backend/internal/crypto` 并由本地 `backend/cmd/crypto-worker` 调用；远程 server 只使用算法目录和非秘密元数据约束，不接触文件明文或 DEK。Electron 不实现密码算法。

## 实施阶段

### 阶段 A：数据与安全基础

1. 新增显式、可重复执行的 `011` 迁移及 Gorm 实体。
2. 扩展存储接口，使密文对象以租户、任务和随机对象键保存，不通过静态路由公开。
3. 落地数据库审计记录器、孤儿对象记录和清理命令。
4. 新增 `crypto.key.self.manage`、`crypto.key.manage` 权限并配置 DO、DU、租户管理员默认授权。
5. 实现 Redis 请求频率限制、用户/租户并发租约、终态释放和异常租约巡检。

### 阶段 B：算法目录和 RSA 公钥版本

1. 建立算法能力目录和租户启用配置，首期只启用 `RSA-OAEP-SHA256`。
2. 实现客户端 RSA 3072 位密钥生成和安全存储。
3. 实现公钥登记、查询、指纹去重、版本和禁用接口。
4. 实现同租户有效接收者和公钥版本查询。

### 阶段 C：服务端任务与密文协议

1. 实现幂等任务创建和不可变授权快照。
2. 实现合法状态转换、进度、取消和重试执行。
3. 实现流式密文上传、服务端哈希复核和暂存对象。
4. 实现完成事务、重复完成、失败补偿、文件列表、详情和鉴权下载。

### 阶段 D：Electron 本地加密闭环

1. 实现受限文件选择、随机文件句柄和选择后校验。
2. 实现本地 Go Crypto Worker、统一 `CryptoEngine`、版本化容器、AES-GCM 流、SHA-256、`RSAEngine` 和 DEK 清理。
3. 实现主进程任务协调、上传、进度、取消、异常恢复和临时密文清理。
4. 实现渲染层页面、算法与接收者动态表单、确认、进度、列表、详情、下载和重试。

### 阶段 E：验证与可读性

1. 完成密码学已知输入、往返、篡改、错误私钥、nonce 唯一性和适配器隔离测试。
2. 完成任务状态、幂等、事务并发、跨租户、对象补偿和敏感字段测试。
3. 完成 1 GiB 性能、内存、UI 响应、取消和 3 并发任务基准。
4. 完成端到端、安全日志扫描和关键注释与可读性检查。

## Phase 1 设计后宪章复核

- 数据模型将 RSA 专有关系放入 `rsa_protected_key_bindings`，通用任务和受保护密钥模型保持算法无关：通过。
- 密文协议明确 AES-GCM 单消息流与 RSA-OAEP-SHA-256 只封装 DEK：通过。
- OpenAPI 和 IPC 契约不传输明文 DEK或私钥，服务端上传只接受密文：通过。
- Benchmark 数据分别保存 AES、RSA、上传和内存指标：通过。
- Audit 为真实持久化实现，不继续使用 `NoopAuditRecorder` 承担本功能安全事件：通过。
- 本期无模拟 CP-ABE；下一期必须优先评估并接入真实 Go TKN20，仅在形成不适用研究结论后才可选择真实 Go BSW07 或其他方案：通过。

## 复杂度跟踪

无宪章违规需要例外说明。
