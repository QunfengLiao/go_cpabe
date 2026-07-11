# 后端租户级 RBAC 基线回归清单

本清单用于 T001，记录实现前需要保持或迁移的现有角色与鉴权链路。后续实现每完成一个阶段，都应回看这些基线，确认没有把旧行为误删为不可恢复状态。

## 表与模型基线

- `roles` 当前由 `backend/internal/domain/tenant.go` 的 `Role` 表示，旧字段包括 `code`、`name`、`scope`、`description`、时间字段和软删除字段；旧唯一约束是全局 `code`。
- `user_roles` 当前由 `UserRoleAssignment` 表示，`tenant_id` 为空表示平台授权，非空表示租户授权；旧撤销行为依赖物理删除或软删除记录释放唯一约束。
- 仓库中没有真实 `tenant_roles` 表，`tenantRoles` 只是前端上下文 DTO 字段，不得新增重复授权事实源。

## 初始化基线

- `TenantService.EnsureBaseRoles` 幂等写入 `PLATFORM_ADMIN`、`TENANT_ADMIN`、`DO`、`DU`。
- `TenantService.BootstrapDefaultTenant` 负责演示租户、默认租户和历史用户迁移。
- `PlatformRoleService.EnsurePlatformAdmin` 写入平台管理员授权。
- `PlatformRoleService.AssignTenantAdmin` 写入租户管理员授权。

## 角色分配与互斥基线

- 旧成员业务角色接口为 `PUT /api/v1/tenants/:id/members/:userId/role`。
- 调用链为 `TenantHandler.AssignTenantMemberRole -> TenantService.AssignTenantMemberBusinessRole -> GormTenantRepository.ReplaceTenantBusinessRole`。
- 旧 `ReplaceTenantBusinessRole` 会删除同租户同成员的 `DO/DU` 后再插入新角色，是本阶段必须移除的互斥行为。

## 租户上下文基线

- 登录用户 ID 来自 `AuthRequired` 校验后的 `middleware.ContextUserID`。
- 租户 ID 来自 `TenantRequired` 读取 `X-Tenant-Id` 并调用 `TenantService.ResolveTenantContext` 校验后的 `middleware.ContextTenantID`。
- 当前实现允许平台管理员仅凭平台身份进入任意启用租户上下文，本阶段需要改为必须满足租户成员与租户角色规则。

## 硬编码鉴权基线

- `TenantService.permissionsForRoles` 按固定角色拼接轻量权限，需迁移为数据库权限事实源。
- `TenantService.ensureTenantManager`、`ensurePlatformOrLegacyAdmin` 仍以角色判断作为管理权限来源。
- `PolicyService` 使用 `TENANT_ADMIN/DO` 判断策略读写。
- `OrgAttributeService` 和 `OrgManagementService` 使用 `TENANT_ADMIN/DO/DU` 或 `requireTenantAdmin` 判断组织能力。
- `PlatformService` 中最后一名租户管理员保护属于角色语义判断，应保留但适配状态化授权。

## 回归路径

- 登录、刷新、退出和 `/api/v1/me/context` 不回归。
- 租户切换仍只接受后端校验过的 `X-Tenant-Id`。
- 旧组织与策略基础接口保持租户边界校验。
- 新增 RBAC 后，旧单角色接口如暂时保留，不得再制造 `DO/DU` 互斥。
