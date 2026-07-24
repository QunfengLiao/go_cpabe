# 实现计划：租户数据批量导入

**分支**：`014-tenant-batch-import` | **日期**：2026-07-17 | **规格**：[spec.md](spec.md)

## 摘要

在现有租户、用户、组织、角色、属性和审计能力上新增批次化 Excel 导入。后端负责模板、解析、完整预校验、批次绑定、确认重校验和批次原子事务；前端提供统一四步导入体验，并在完成后刷新现有管理页面。

## 技术上下文

**语言/版本**：Go 1.23；TypeScript + React 19

**主要依赖**：Gin、Gorm、MySQL、现有认证/RBAC/组织属性服务、兼容 Go 1.23 的 `github.com/xuri/excelize/v2 v2.9.1`、Ant Design 5

**存储**：MySQL；临时上传文件使用本地临时目录并请求结束清理

**测试**：Go `go test ./...`；前端 Vitest、TypeScript 类型检查和 Vite 构建

**目标平台**：后端服务与 Electron 桌面端

**项目类型**：桌面端 + HTTP API

**性能目标**：在后端配置最大行数内批量预取依赖数据，避免单行重复查询；模板和预览响应可分页扩展

**约束**：只接受 `.xlsx`；大小、行数和批次有效期由后端配置；正式导入一个批次一个事务；密码不持久化到批次和日志

**规模**：首期用户和组织两种导入类型，复用现有租户隔离和权限规则

## 宪章检查

- **混合加密边界**：不涉及文件、DEK、AES-GCM、RSA-OAEP 或 CP-ABE，导入功能不改变加密模块。
- **真实 CP-ABE 实现**：不新增密码学实现，不使用模拟加解密。
- **模块边界**：新增 Import Domain/Repository/Service/Handler；用户、组织、角色、属性和 Audit 仍通过既有边界协作。
- **算法对比口径**：本功能不产生 benchmark，不混入 AES 或 DEK 指标。
- **可解释性**：通过行号、字段、动作、错误码、统计和批次审计解释导入结果。
- **中文文档**：本功能 SpecKit 文档全部使用简体中文，代码标识符保留英文。
- **Go 注释策略**：所有新增或修改的 Go 函数/方法前补充中文职责与安全/事务边界注释；导出标识符使用 GoDoc 前缀；实体字段说明租户来源、敏感性和外部响应；Handler/Service/Repository 说明副作用和错误边界。
- **关键注释和可读性检查**：tasks.md 明确安排最终检查，覆盖函数注释、GoDoc、实体字段、租户隔离、文件安全、事务取舍和无意义注释清理。

## 企业级用户模板设计

- **兼容布局**：第 1 行继续保存原有系统字段 key 并隐藏；第 2 至 4 行依次为合并标题、填写说明和中文展示表头；第 5 行起为示例与管理员数据。解析器识别该装饰区，同时继续兼容旧版第 2 行直接开始数据的文件。
- **工作表结构**：用户模板固定包含“用户导入”“字段说明”“数据字典”；组织模板和错误报告沿用现有数据语义，但统一由 `excelize` 生成。
- **视觉语言**：采用深蓝主标题、蓝色表头、浅色说明区、克制的浅灰边框和统一中文字体；隐藏网格线，设置合理列宽、行高、冻结窗格和筛选区域。
- **录入约束**：`role_codes` 列提供 `DO`、`DU`、`DO,DU`、`TENANT_ADMIN` 下拉，`member_status` 列提供 `ACTIVE`、`DISABLED` 下拉；下拉只辅助录入，服务端仍执行最终权限和租户边界校验。
- **代码边界**：样式、列宽和数据校验分别封装为 `createTitleStyle`、`createHeaderStyle`、`setColumnWidths`、`addValidation` 等私有函数，业务 Service 只传入模板定义，不堆积样式细节。

## 大批量校验与上传交互修正

- 默认 `IMPORT_MAX_ROWS` 调整为 10000，使系统下载和实际导入容量一致；超过限制时返回独立错误码，前端持久显示原因。
- 初始密码只在请求内存中短暂存在；结构校验和用户动作识别完成后，仅为合法的新增用户执行 bcrypt，并使用受限工作池并发计算，摘要仍写入可信批次快照，明文不进入响应、日志或数据库。
- 导入抽屉使用固定步骤区和独立滚动内容区；已选文件、进度/错误提示和操作区分块布局，避免 Ant Design 上传列表与按钮重叠。
- 上传主按钮改为“校验并进入预览”，校验中显示持久状态，成功后自动进入下一步，失败后错误留在当前页面并允许重试。

## 结构决策

```text
backend/
├── internal/domain/import.go
├── internal/repository/import_repository.go
├── internal/service/import_service.go
├── internal/handler/import_handler.go
├── internal/handler/router.go
├── internal/config/config.go
├── migrations/017_tenant_import.sql
└── go.mod

desktop/src/renderer/src/
├── api/import.ts
├── components/tenant-import/TenantImportDrawer.tsx
├── pages/TenantMembersPage.tsx
└── pages/TenantOrgManagementPage.tsx
```

**结构选择**：保持现有 `backend/internal/{domain,repository,service,handler}` 分层和 Electron renderer 的 API/component/page 分层；不创建重复的用户或组织实体。

## 复杂度记录

| 事项 | 原因 | 已拒绝的更简单方案 |
|---|---|---|
| 批次行 JSON | 需要在预校验和确认间保存可信快照并支持详情/报告 | 只存前端临时数据无法防篡改和刷新恢复 |
| 导入专用事务协作函数 | 必须共享一个事务并保持现有角色/属性业务规则 | Handler 直接写关联表会破坏边界和原子性 |

## 万级异步导入增量设计

### 技术方案

- 确认接口使用不包含 `rows_json` 的轻量元数据查询，只执行批次归属、有效期、状态和预校验统计检查，并将 `VALIDATED` 原子转换为 `QUEUED`；重复确认返回已有状态。
- 后端新增基于 MySQL 持久状态的 `ImportWorker`。Worker 使用租约令牌领取 `QUEUED` 或租约已过期的 `IMPORTING` 批次，定期写入心跳和阶段进度，服务重启后可重新领取。
- Worker 在业务事务开始前加载完整快照，将 MySQL JSON 返回内容反序列化为强类型行结果后重新序列化，再与预校验摘要比较；该规范化校验兼容数据库调整键顺序和空白，但字段值、类型或结构变化仍会被判定为篡改。
- Worker 领取事务只以 `id` 窄行投影执行排序和 `FOR UPDATE SKIP LOCKED`，提交随机租约后再按主键加载 `rows_json`；避免大快照进入 MySQL filesort，也缩短领取事务持锁时间。
- 用户正式写入在一个业务事务中按 200～500 行分批构造 SQL：批量 UPSERT 用户和租户成员，批量替换角色，批量建立组织关系和属性关系，避免每行重复读取相同组织、角色和租户数据。
- 批次状态与心跳在业务事务之外更新；最终业务写入仍保持全成或全败。任务失败时记录稳定错误码和脱敏原因，不保存原始 SQL。
- 新增轻量状态接口，轮询响应只包含状态、阶段、统计和进度，不返回完整行快照；现有详情和错误报告接口继续受租户与操作者边界保护。
- Electron 前端确认后立即进入处理页，每 1.5 秒轮询轻量状态；持久展示请求错误、处理中阶段和进度条，完成后刷新管理页面。
- Worker 空队列领取使用 `Find + Limit(1)` 表达正常无任务状态，并仅静默这条预期探测；真实 SQL 错误继续向 Worker 返回。默认空闲轮询间隔为 5 秒，避免日志噪音和每秒数据库往返。

### 数据与迁移

`tenant_import_batches` 增加 `QUEUED` 状态、`phase`、`processed_count`、`lease_token`、`lease_expires_at`、`heartbeat_at` 和 `last_error_code`。迁移必须幂等，并为待领取状态和租约建立索引。

### 安全与幂等

- 批次领取使用状态条件和唯一租约令牌，确保任一时刻只有一个 Worker 执行。
- 用户名、租户成员、角色和组织成员继续依赖数据库唯一约束与 UPSERT；确认接口对 `QUEUED/IMPORTING/SUCCEEDED` 返回当前状态。
- 预校验生成的 bcrypt 摘要只存在服务端批次快照，轻量状态接口和前端永不返回摘要；不得为多个用户复用同一摘要。
- 预校验同时建立用户名和邮箱占用索引，拒绝邮箱属于其他用户名或文件内重复邮箱；正式 UPSERT 后重新加载用户名并断言每行均得到非零用户主键，以覆盖验证和执行之间的并发唯一键冲突。

### 万级成员列表

- 租户成员管理接口增加 `page`、`page_size`，默认每页 50 条且限制最大值；响应返回当前页 `users`、`total`、`page` 和 `page_size`。
- Repository 先分页读取成员关系，再只为当前页批量读取用户和角色；需要完整成员集的内部任务按固定窗口遍历，避免任何 SQL 携带万级 ID。
- Electron 成员页只渲染当前页并提供上一页、下一页和总数；导入完成后刷新当前页与总数。
- 新增幂等迁移只清理 `tenant_users`、`user_roles`、`tenant_org_members`、`tenant_org_member_roles`、`user_attributes` 中 `user_id=0` 的历史非法关系，不自动删除或恢复任何真实用户。

### Go 注释策略

新增或修改的 Worker、Repository、Service 和 Handler 函数均添加中文前置注释；导出标识符使用 GoDoc 前缀；租约、幂等、事务、密码摘要和故障恢复代码使用必要块注释解释安全边界与副作用，避免逐行复述。

### 验证策略

- Repository 测试覆盖原子排队、并发领取、租约过期重领、心跳和终态更新。
- Service 测试覆盖重复确认幂等、后台成功/失败、MySQL JSON 规范化兼容、真实快照篡改、批量用户事务与无明文密码响应。
- Handler/API 测试覆盖确认受理和轻量状态接口。
- 前端测试覆盖确认错误持久化、轮询、进度显示、终态刷新和卸载清理。
- 导入回归覆盖邮箱跨用户名冲突、文件内重复邮箱和 UPSERT 后用户名缺失整体回滚；成员列表回归覆盖默认分页、页大小上限和当前页角色聚合。
- 完成 `go vet ./...`、`go test ./...`、前端类型检查、测试与构建，并执行关键注释和可读性检查。
