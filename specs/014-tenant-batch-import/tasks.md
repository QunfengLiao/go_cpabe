# 任务：租户数据批量导入

**输入**：`spec.md`、`plan.md`、`research.md`、`data-model.md`、`contracts/import-api.md`

## Phase 1：基础设置

- [x] T001 [P] 评估 Excel 依赖；初版采用标准库 ZIP/XML 验证最小导入闭环
- [x] T002 [P] 在 `backend/internal/config/config.go` 增加导入文件大小、最大行数、批次 TTL、临时目录配置
- [x] T003 [P] 在 `backend/migrations/017_tenant_import.sql` 创建批次表并在 `backend/internal/migrations/sql_migrations.go` 接入迁移

## Phase 2：基础能力

- [x] T004 在 `backend/internal/domain/import.go` 增加导入类型、状态、批次、行结果、错误和统计实体，补齐中文字段注释与表名
- [x] T005 [P] 在 `backend/internal/repository/import_repository.go` 增加带 `tenant_id` 条件的批次创建、更新、查询、过期和行结果存取
- [x] T006 [P] 实现模板字段定义、错误报告安全转义和 SHA-256 工具
- [x] T007 在 `backend/internal/service/import_service.go` 建立模板、解析、预校验、确认和批次查询服务接口，明确事务与审计边界
- [x] T008 [P] 在 `backend/internal/handler/import_handler.go` 建立租户上下文、管理员权限、multipart 文件限制和统一错误响应骨架
- [x] T009 修改 `backend/internal/handler/router.go` 注册导入接口，并扩展 `backend/cmd/server/main.go` 依赖装配

## Phase 3：用户故事 1——组织架构导入（P1）

- [x] T010 [P] [US1] 增加组织校验核心测试
- [x] T011 [US1] 实现组织模板、Excel 解析、标题/字段/枚举/负责人校验和逐行错误结果
- [x] T012 [US1] 实现组织父子依赖拓扑排序、组织指纹检查和新增/更新预览动作
- [x] T013 [US1] 实现组织批次确认事务，复用组织属性值规则并阻止非法停用/删除语义
- [x] T014 [US1] 完成组织模板、validate、confirm 和错误报告 HTTP 接口

## Phase 4：用户故事 2——租户用户导入（P1）

- [x] T015 [P] [US2] 增加用户导入校验核心测试
- [x] T016 [US2] 实现用户模板、解析、格式校验、属性解析和批量依赖索引
- [x] T017 [US2] 实现用户新增/加入/更新预览动作和批次快照完整性校验
- [x] T018 [US2] 实现用户确认原子事务，调用现有租户角色分配语义并同步组织成员和用户属性
- [x] T019 [US2] 完成用户模板、validate、confirm 和错误报告 HTTP 接口

## Phase 5：用户故事 3——批次与前端体验（P2）

- [x] T020 [P] [US3] 覆盖 batch_id 跨用户、跨租户、过期、重复确认、篡改和错误报告安全转义
- [x] T021 [US3] 实现批次列表、详情、错误报告及模板/上传/确认/完成/失败审计事件
- [x] T022 [P] [US3] 增加模板下载、上传校验、确认、批次查询和错误报告下载客户端
- [x] T023 [US3] 实现四步导入、统计卡片、错误筛选、动作筛选、防重复提交和结果页
- [x] T024 [US3] 修改成员管理和组织管理页面增加批量导入按钮并在成功后刷新数据
- [x] T025 [P] [US3] 通过 API 客户端测试覆盖导入请求的租户路径与批次参数边界

## Phase 6：收尾与跨切面质量

- [x] T026 [P] 补充导入 Handler 的权限边界和错误映射测试
- [x] T027 [P] 补充导入迁移结构检查
- [x] T028 执行 `gofmt`、`go vet`、`go test ./...`、前端 `npm run typecheck`、`npm test -- --run` 和 `npm run build`，修复回归
- [x] T029 在所有新增或修改 Go 业务代码中执行关键注释和可读性检查：确认每个函数/方法有前置中文注释，导出标识符符合 GoDoc 前缀，实体字段/Handler/Service/Repository 解释业务语义、事务副作用和安全边界，并移除无意义逐行注释
- [x] T030 更新 `backend/README.md`、`desktop/README.md` 和 `specs/014-tenant-batch-import/quickstart.md`，记录配置、接口和已知风险

## Phase 7：企业级用户导入模板

- [x] T031 [P] 更新规格与计划，定义隐藏系统字段行、可见中文表头、三工作表结构和旧版模板兼容边界
- [x] T032 用 `github.com/xuri/excelize/v2 v2.9.1` 替换手写工作簿生成与解析，保留公式拒绝、最大行数和真实 Excel 行号
- [x] T033 将标题、表头、说明、边框、列宽、冻结窗格、自动筛选和数据校验封装为独立 Excel 辅助函数
- [x] T034 [P] 补充用户模板工作表、隐藏字段、中文表头、字段说明、数据字典、下拉校验和旧模板兼容测试
- [x] T035 生成真实用户模板并完成内容检查、工作簿渲染、视觉修复、GoDoc/关键注释与安全边界检查

## Phase 8：万级导入交互与性能修正

- [x] T036 修复导入抽屉滚动区、文件信息和操作区布局，消除遮挡并补充响应式样式
- [x] T037 增加校验中和校验失败的持久反馈，将主操作明确为“校验并进入预览”
- [x] T038 将默认最大行数调整为 10000，并为超限场景增加稳定服务端错误码
- [x] T039 将新增用户的 bcrypt 预处理改为安全边界内的受限并发，跳过无效行和更新用户
- [x] T040 补充配置、错误映射、密码预处理测试并执行前后端回归与关键注释检查

## 依赖与执行顺序

- Phase 1 → Phase 2 → US1/US2 → US3 → Phase 6。
- US1 与 US2 共享导入基础设施，不能在 T007/T009 前开始；组织与用户服务实现可在基础完成后分别推进。
- T022、T025、T026、T027 可并行；涉及同一 Go 文件的任务必须顺序执行。

## Phase 9：万级异步导入与可观察进度（US4，P1）

- [x] T041 [P] [US4] 更新 `backend/migrations/017_tenant_import.sql`、`backend/internal/migrations/import_migration_test.go` 和 `backend/internal/domain/import.go`，增加排队、阶段、进度、租约、心跳与稳定错误字段
- [x] T042 [P] [US4] 在 `backend/internal/repository/import_repository.go` 增加幂等排队、租约领取、心跳续租、进度和终态持久化能力及仓储测试
- [x] T043 [US4] 在 `backend/internal/service/import_worker.go` 实现持久化 Worker、租约恢复、批次执行和脱敏失败边界
- [x] T044 [US4] 重构 `backend/internal/service/import_service.go`，让确认接口快速入队并保持重复确认幂等
- [x] T045 [US4] 在 `backend/internal/service/import_bulk_user.go` 实现用户、租户成员、角色和组织成员的事务内批量 UPSERT，消除万级逐行查询链路
- [x] T046 [P] [US4] 在 `backend/internal/handler/import_handler.go` 与 `backend/internal/handler/router.go` 增加 HTTP 202 受理语义和轻量批次状态接口
- [x] T047 [US4] 在 `backend/cmd/server/main.go` 装配 Worker 生命周期，并在 `backend/internal/config/config.go` 增加批量大小、轮询和租约配置
- [x] T048 [P] [US4] 在 `desktop/src/renderer/src/api/import.ts` 增加 `QUEUED`、进度模型和轻量状态客户端，并补充 API 测试
- [x] T049 [US4] 重构 `desktop/src/renderer/src/components/tenant-import/TenantImportDrawer.tsx`，增加确认错误持久化、后台轮询、进度条、关闭恢复和终态刷新
- [x] T050 [P] [US4] 补充后端重复确认、租约恢复、Worker 成功/失败、批量 SQL 有界性和无密码泄漏测试
- [x] T051 [US4] 执行后端与前端完整回归、万行导入验证，并更新 `backend/README.md`、`desktop/README.md` 和本 quickstart
- [x] T052 [US4] 执行关键注释和可读性检查：确认所有 Go 函数/方法有中文前置注释、导出标识符符合 GoDoc 前缀、实体字段与 Worker/事务/租约/密码安全边界说明完整且无无意义逐行注释
- [x] T053 [US4] 修复空队列 `record not found` 日志噪音，将领取查询改为无错误空结果语义并把默认轮询间隔调整为 5 秒
- [x] T054 [US4] 修复万行确认同步加载 `rows_json` 和 MySQL JSON 规范化导致的快照误判：确认改用轻量元数据入队，Worker 使用规范 JSON 校验摘要，并补充回归测试
- [x] T055 [US4] 修复 Worker 领取万行批次时 `SELECT *` 排序导致的 MySQL `Out of sort memory`：事务内只排序锁定主键，租约提交后按主键加载完整快照

## Phase 10：导入身份完整性与万级成员分页

- [x] T056 [P] 增加邮箱跨用户名占用、文件内重复邮箱和 UPSERT 后用户名未解析的导入回归测试
- [x] T057 修复用户预校验和正式导入身份完整性边界，任何用户主键缺失时整体回滚并禁止 `user_id=0`
- [x] T058 [P] 增加租户成员默认分页、页大小边界、当前页角色聚合和前端分页 API 测试
- [x] T059 实现租户成员 Repository/Service/Handler 服务端分页，并让 Electron 成员页只加载和渲染当前页
- [x] T060 增加幂等数据修复迁移，精确清理历史 `user_id=0` 导入关系并接入迁移测试
- [x] T061 执行前后端完整回归、构建、迁移检查与关键注释和可读性检查，并更新 README、接口契约和 quickstart

## MVP 范围

完成 Phase 1、Phase 2、US1 和 US2 后即可演示两个真实导入闭环；US3 提供历史批次、错误报告和完整前端体验，必须在交付前完成。
