# 任务：租户管理员新增成员

## 阶段 1：基础

- [X] T001 核对现有用户、成员、角色、权限和审计边界，更新 `specs/011-tenant-member-create/plan.md`
- [X] T002 [P] 定义新增成员接口契约和复用规则，更新 `specs/011-tenant-member-create/contracts/openapi.yaml`

## 阶段 2：用户故事 1——创建或复用成员

- [X] T003 [P] [US1] 在 `backend/internal/service/tenant_service_test.go` 添加新账号、已有账号、非法角色和越权测试
- [X] T004 [P] [US1] 在 `backend/internal/handler/tenant_member_create_test.go` 添加当前租户接口契约测试
- [X] T005 [US1] 在 `backend/internal/service/tenant_service.go` 实现输入校验、bcrypt 初始密码、账号复用、成员角色和审计规则
- [X] T006 [US1] 在 `backend/internal/handler/tenant_handler.go` 与 `backend/internal/handler/router.go` 接入 `POST /api/v1/tenant/members`
- [X] T007 [P] [US1] 在 `desktop/src/renderer/src/api/tenant.ts` 和测试中添加创建成员请求
- [X] T008 [US1] 在 `desktop/src/renderer/src/pages/TenantMembersPage.tsx` 添加新增成员表单、角色选择和临时密码提示

## 阶段 3：用户故事 2——接收者集成

- [X] T009 [US2] 验证新 DU 登记公钥后进入接收者目录，补充 `backend/internal/service/rsa_key_service_test.go`
- [X] T010 [P] [US2] 在 `desktop/src/renderer/src/pages/TenantMembersPage.test.tsx` 覆盖创建与刷新交互

## 阶段 4：收口

- [X] T011 运行后端、桌面端全量测试并记录到 `specs/011-tenant-member-create/quickstart.md`
- [X] T012 执行关键注释、GoDoc、安全边界和敏感信息检查，更新 `specs/011-tenant-member-create/checklists/requirements.md`

## 依赖

T001–T002 → T003–T004 → T005–T008 → T009–T010 → T011–T012。

## MVP

T001–T008 构成可独立验收的租户管理员新增成员闭环。
