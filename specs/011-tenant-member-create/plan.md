# 实施计划：租户管理员新增成员

## 摘要

扩展现有租户成员管理链路：在可信当前租户接口下创建或复用用户，建立成员关系，授予 `DO`/`DU` 角色并记录审计；桌面端成员页增加表单和一次性初始密码提示。已有账号按邮箱复用且永不重置密码。

## 技术上下文

- 后端：Go、Gin、Gorm、MySQL，复用 `users`、`tenant_users`、`user_roles`。
- 桌面端：Electron、TypeScript、React，复用现有成员页和请求封装。
- 权限：`tenant.member.manage`；租户 ID 只取鉴权中间件上下文。
- 密码：新账号使用 bcrypt 摘要保存 `lqf999..`，`must_change_password=true`。
- 测试：Service、Handler 契约和 React/请求单元测试。

## 宪章检查

- 不修改 Crypto、Policy、Benchmark 或混合加密边界：通过。
- User、租户 RBAC 和 Audit 模块职责保持分离：通过。
- 所有 SpecKit 文档使用简体中文：通过。
- Go 注释策略：所有新增或修改函数/方法添加前置中文注释；Handler、Service 说明租户鉴权、密码哈希、账号复用、写入副作用与审计失败边界；导出标识符使用 GoDoc 前缀。

## 设计

1. 新增当前租户 `POST /api/v1/tenant/members`，由租户中间件提供可信租户。
2. Service 规范化并校验输入，按邮箱查找用户；不存在则哈希初始密码并创建账号，存在则保持原资料和密码。
3. 建立或恢复成员关系，授予去重后的 `DO`/`DU`，写入 `tenant_member.account_created` 或 `tenant_member.account_reused` 审计。
4. 返回成员聚合、`created_user` 和新建账号的一次性 `temporary_password`。
5. React 表单提交成功后刷新成员列表；已有账号不展示临时密码。

## 风险与控制

- 固定密码仅适用于演示，必须强制首次改密并禁止日志记录。
- 跨租户风险由当前租户中间件和权限中间件双重限制。
- 账号复用不能把表单字段写回已有账号，避免越权修改身份。
- 本期沿用现有多仓储写入方式；任何后续生产化必须将账号、成员和角色写入收口为数据库事务。
