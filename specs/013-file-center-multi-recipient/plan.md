# 实施计划：多接收者文件中心与本地解密闭环

**分支**：`013-file-center-multi-recipient` | **日期**：2026-07-12 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/013-file-center-multi-recipient/spec.md` 的功能规格。

## 摘要

本功能将当前 RSA+AES 链路收口为客户端加密模型：DO 仍选择普通明文文件，但 Electron 主进程/本地 Crypto Worker 只在本地读取明文，生成一份 AES-256-GCM DEK 和一份文件密文，再为每个目标 DU 生成一份 RSA-OAEP-SHA256 密钥信封。后端只接收密文流、非敏感容器元数据和密钥信封，保存并返回原始密文，不接触明文或原始 DEK。

本次修订将“文件可见性”和“本地解密能力”彻底分离：有效租户成员即可进入文件中心、查看全部密文元数据、下载原始密文和信封；`file.read`、`file.decrypt.invoke`、接收者关系和服务端状态不得作为这些读取接口的门槛。上传、删除、重命名、公钥管理和管理审计仍由 RBAC 控制。Electron 主进程从本地安全存储读取私钥并逐个匹配信封，服务端不保存私钥、原始 AES 密钥，不执行解密或返回明文。

## 技术上下文

**语言/版本**：Go 1.23、TypeScript、Electron、React。  
**主要依赖**：Gin、Gorm、MySQL、Redis、Electron IPC、Ant Design、Node.js 流式文件能力、Go crypto 标准库。  
**存储**：MySQL 保存文件、任务、密文对象、受保护 DEK、RSA 绑定、性能指标、正式审计和审计 outbox；Redis 用于任务准入和令牌。  
**测试**：Go `go test`、桌面端 `vitest`、TypeScript `tsc --noEmit`、Electron 主进程单元测试和 worker e2e 测试。  
**目标平台**：Windows 桌面端 + 本地 Go Crypto Worker + Go HTTP 服务。  
**项目类型**：Electron + React 桌面应用，Go + Gin + Gorm 后端服务。  
**性能目标**：进度来自真实字节数或接收者计数；性能指标可拆分 AES、DEK 保护、上传、元数据提交和总耗时。  
**约束**：明文文件、明文 DEK、RSA 私钥和本地完整路径不得上传服务端；可见用户可以获取密文包和受保护密钥信封，但不得获取私钥、明文 DEK或明文；不得使用 RBAC 角色推断最终解密结果；不得使用伪造进度。  
**规模/范围**：本期覆盖 RSA-OAEP-SHA256 多接收者，保留 CP-ABE 扩展边界，不接入新的 CP-ABE 库。

## 宪章检查

*门禁：Phase 0 研究前必须通过；Phase 1 设计后复核。*

- **混合加密边界**：通过。文件内容仍由 AES-GCM 加密一次；RSA-OAEP 只封装 DEK。多接收者只增加多份受保护 DEK，不重复加密文件内容。
- **真实 CP-ABE 实现**：通过。本期不声称实现 CP-ABE，也不使用模拟 CP-ABE；只修正 RSA 基线，并保留算法注册扩展。
- **模块边界**：通过。Crypto 负责 AES-GCM、RSA-OAEP 和 worker 协议；File 负责文件列表、详情、下载和解密材料授权；Benchmark 负责分项指标；Audit 负责安全事件；Handler 不承载密码算法。
- **解密权限边界**：通过。File 模块只校验有效租户成员、文件和对象状态；密文下载与密钥信封读取不按接收者或角色筛选，是否存在本地匹配私钥及是否能完成解密只在 Electron 主进程与本地 Worker 判定。
- **算法对比口径**：通过。计划新增 AES 耗时、DEK 保护耗时、单接收者 RSA 耗时、上传耗时和总耗时拆分，避免把网络上传混入算法结论。
- **可解释性**：通过。文件中心只展示文件名、拥有者、算法、密文大小、创建时间和下载操作；详情可以展示密文元数据与信封摘要，不展示“可解密/未授权”推断。
- **中文文档**：通过。所有 SpecKit 文档使用简体中文；代码标识符、接口字段和路径保留工程英文。
- **Go 注释策略**：通过。所有新增或修改 Go 函数/方法必须补充中文前置注释；导出标识符使用 GoDoc 前缀；实体字段、Handler、Service、Repository、Middleware 注释说明业务语义、副作用、鉴权和安全边界，重点解释密钥信封不是 RBAC 解密授权、私钥仅存在本地安全存储以及失败密文保留策略。
- **关键注释和可读性检查**：通过。`tasks.md` 必须包含专门任务，检查函数/方法注释、GoDoc 前缀、核心模块业务语义注释、安全边界和无意义注释清理。
- **审计可靠性**：通过。安全关键业务写入与 `audit_outbox` 同事务，Dispatcher 只负责幂等投递；outbox 不保存秘密且不复用文件孤儿清理边界。
- **客户端加密注释策略**：通过。新增或修改的 Go Service、Repository、Handler、Crypto 和上传/下载代码必须说明“服务端只接触密文”的安全边界；Electron 私钥、DEK、文件明文和解密失败保留密文逻辑由主进程注释说明，Renderer 只能调用受控 IPC。

## 项目结构

### 本功能文档

```text
specs/013-file-center-multi-recipient/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── openapi.yaml
│   └── desktop-ipc.md
└── tasks.md
```

### 源码结构

```text
backend/
├── migrations/
│   ├── 012_multi_recipient_file_center.sql
│   └── 014_audit_outbox.sql
├── internal/domain/
│   ├── encryption_key.go
│   └── encryption_task.go
├── internal/repository/
│   ├── encryption_repository.go
│   └── rsa_key_repository.go
├── internal/service/
│   ├── encryption_service.go
│   ├── audit_recorder.go
│   ├── audit_dispatcher_service.go
│   ├── encryption_algorithm_adapter.go
│   ├── encrypted_file_service.go
│   └── rsa_key_service.go
├── internal/handler/
│   ├── encryption_handler.go
│   ├── encrypted_file_handler.go
│   └── router.go
└── internal/crypto/
    ├── content_cipher.go
    └── rsa_engine.go

backend/cmd/audit-dispatcher/
└── main.go

desktop/
├── src/main/encryption/
│   ├── coordinator.ts
│   ├── decryptionCoordinator.ts
│   ├── taskApiClient.ts
│   ├── types.ts
│   └── cryptoWorkerProtocol.ts
├── src/preload/preload.ts
└── src/renderer/src/
    ├── api/encryption.ts
    ├── components/encryption/
    ├── pages/DataEncryptionPage.tsx
    ├── pages/FileCenterPage.tsx
    ├── pages/EncryptedFilesPage.tsx
    ├── pages/ReceivedFilesPage.tsx
    └── utils/format.ts
```

**结构决策**：沿用现有后端模块边界和桌面端主进程/预加载/渲染进程分层；新增文件中心页面与格式化工具，但保留旧路由页面作为兼容跳转壳。数据库复用 `protected_keys` 与 `rsa_protected_key_bindings` 表并调整唯一约束，不新建重复的 recipient_ids 字符串字段。

### 审计 Outbox 一致性边界

- 安全关键业务写操作由 Repository 在同一 Gorm/MySQL 事务内写业务事实和已过滤的 `audit_outbox` 事件；任何一步失败都回滚。
- 拒绝、下载和客户端进度等无业务写事务事件使用短事务入队；若 MySQL 整体不可用，则只输出不含 `metadata` 的结构化告警并增加监控计数，不把秘密写入普通日志。
- 入队前按 `action` 使用类型化字段规则校验；未知或非法 Metadata 不进入 outbox，系统保留顶层最小事件并标记 `metadata_redacted`。普通失败日志只记录事件 UUID、动作和稳定错误分类，不输出 `err.Error()`、请求体字段或内部对象路径。
- Dispatcher 使用 `SELECT ... FOR UPDATE SKIP LOCKED` 领取有限批次并写入 `locked_at`、`lock_token` 处理租约，按 `event_public_id` 向 `audit_logs` 幂等插入；成功标记 `DELIVERED`，失败按指数退避进入 `RETRY`，超过上限进入 `DEAD_LETTER`。完成和失败更新必须携带 `lock_token`，防止过期 Worker 覆盖新租约。
- `audit_logs` 增加 `metadata_redacted` 事实字段，使被清空 Metadata 的最小事件在正式日志中仍可解释，而不是把该标记重新塞回不可信 JSON。
- `PROCESSING` 租约超时后允许重新领取；重复投递依靠 `audit_logs.public_id` 唯一约束收敛，不依赖进程内锁。
- `event_public_id` 解决 Dispatcher 至少一次投递重复；可选 `dedup_key` 解决生产者对同一业务事实的重复发射，两者职责分离。
- 已投递记录按保留期清理，死信只允许专用审计命令受控重放；文件孤儿清理命令永远不查询 `audit_outbox`。
- OpenAPI 与 Electron IPC 无新增公共接口；outbox 是后端内部可靠性协议，详见 `contracts/audit-outbox.md`。

## 复杂度跟踪

| 复杂度 | 为什么需要 | 拒绝的更简单方案 |
|--------|------------|------------------|
| 修改 protected key 一对多结构 | 单文件多接收者要求每个接收者一份受保护 DEK | 文件表拼接 recipient_ids 无法表达公钥版本、单接收者耗时和后端鉴权 |
| Worker 支持多接收者 RSA-OAEP | 文件只能 AES 加密一次，但 DEK 需要多次封装 | 为每个接收者重复加密并上传整份密文违反规格和性能目标 |
| 文件中心统一密文查询 | 全部密文和我的文件共享详情与租户边界 | 继续保留分散页面会重复查询逻辑并混入解密授权推断 |
| 独立审计 outbox 与 Dispatcher | 业务事务和审计表写入需要可恢复、幂等且不能污染文件清理队列 | 直接尽力写日志会静默丢失；复用孤儿对象表会被误删除且无法重放完整事件 |

## Phase 0 输出

已生成 [research.md](./research.md)。

## Phase 1 输出

已生成 [data-model.md](./data-model.md)、[contracts/openapi.yaml](./contracts/openapi.yaml)、[contracts/desktop-ipc.md](./contracts/desktop-ipc.md)、[quickstart.md](./quickstart.md)。

## 宪章复核

- 混合加密边界在数据模型中体现为一份 `CiphertextObject` 对多份 `ProtectedDEK`。
- 真实 CP-ABE 未被模拟；RSA 多接收者作为基线能力实现。
- 模块边界在接口契约和任务分解中保持清晰。
- Benchmark 拆分满足 RSA 与后续 CP-ABE 对比口径。
- Go 注释策略已写入计划，并将在任务阶段列为最终检查。
- 审计 outbox 在入队前复用正式审计白名单，状态机、租约和死信字段均不包含秘密；Audit 与 File 补偿边界保持独立。
