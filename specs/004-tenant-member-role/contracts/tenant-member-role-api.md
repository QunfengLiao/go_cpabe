# 接口契约：租户成员角色分配

## PUT /api/v1/tenants/:tenantId/members/:userId/role

为目标租户中的有效成员分配一个普通业务角色。

### 鉴权

- 必须携带有效登录态。
- 当前用户必须是路径 `tenantId` 对应租户下的 `TENANT_ADMIN`。
- 当前用户不能仅凭 `PLATFORM_ADMIN` 调用本接口。
- 普通成员不能调用本接口。

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `tenantId` | number | 是 | 目标租户 ID |
| `userId` | number | 是 | 被分配角色的租户成员用户 ID |

### 请求体

```json
{
  "roleCode": "DATA_OWNER"
}
```

### 请求字段

| 字段 | 类型 | 必填 | 允许值 | 说明 |
|------|------|------|--------|------|
| `roleCode` | string | 是 | `DATA_OWNER`, `DATA_VISITOR` | 目标普通业务角色 |

### 角色映射

| 请求值 | 展示名称 | 内部角色编码 |
|--------|----------|--------------|
| `DATA_OWNER` | 数据拥有者 | `DO` |
| `DATA_VISITOR` | 数据访问者 | `DU` |

`PLATFORM_ADMIN` 和 `TENANT_ADMIN` 不允许作为本接口请求值。

### 成功响应

沿用系统统一响应格式：

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "user_id": 12,
    "email": "member@example.com",
    "nickname": "成员",
    "member_status": "active",
    "roles": ["DO"]
  }
}
```

### 错误响应

| 场景 | 建议错误码 | 说明 |
|------|------------|------|
| 未登录或 token 无效 | `ACCESS_TOKEN_INVALID` | 登录态无效 |
| 当前用户不是目标租户管理员 | `TENANT_PERMISSION_DENIED` | 无权限操作 |
| 当前用户是平台管理员但不是目标租户管理员 | `TENANT_ROLE_ASSIGN_PLATFORM_FORBIDDEN` | 平台管理员不参与租户内业务角色分配 |
| 目标租户不存在 | `TENANT_NOT_FOUND` | 租户不存在 |
| 目标租户已禁用 | `TENANT_DISABLED` | 禁用租户不能分配普通业务角色 |
| 目标用户不是租户成员 | `TENANT_MEMBER_FORBIDDEN` | 用户不是该租户成员 |
| 目标成员已禁用 | `TENANT_MEMBER_DISABLED` | 成员关系不可用 |
| 修改自己的租户管理员角色 | `TENANT_ADMIN_SELF_ROLE_FORBIDDEN` | 不能修改自己的租户管理员角色 |
| 请求角色不是允许的普通业务角色 | `INVALID_ROLE` | 角色不存在或不允许分配 |
| 事务写入失败 | `INTERNAL` 或现有数据库错误映射 | 不应出现半完成数据 |

## GET /api/v1/tenants/:tenantId/users

复用现有成员列表接口。该接口必须返回成员当前租户角色，供前端角色列和弹窗回显使用。

### 成功响应示例

```json
{
  "code": "OK",
  "message": "success",
  "data": {
    "users": [
      {
        "user_id": 12,
        "email": "member@example.com",
        "nickname": "成员",
        "member_status": "active",
        "roles": ["DU"]
      }
    ]
  }
}
```

### 说明

- 如果角色刚刚保存成功，前端应重新调用成员列表接口刷新。
- 平台后台成员列表可以继续保留平台管理员指定 `TENANT_ADMIN` 的能力，但普通业务角色分配入口不应展示给平台管理员。
- 前端已规划普通租户成员页 `/tenant/members` 使用本接口完成日常业务角色分配；平台后台页面仅保留平台管理员兜底管理能力。
