# 接口契约：租户组织架构管理

## 通用约定

- 所有接口前缀为 `/api/v1`。
- 新组织管理接口统一使用 `/tenant/...`，不在路径或请求体中接收可信 `tenant_id`。
- 所有接口必须携带有效登录态和当前租户上下文。
- 写操作仅允许当前租户 `TENANT_ADMIN`。
- 响应沿用项目统一响应格式，本文只描述 `data` 内结构。
- 旧 `/tenants/:id/...` 组织接口只作为迁移过渡入口，必须复用同一个 Service。

## 查询当前租户部门树

`GET /api/v1/tenant/org-units/tree`

**允许角色**：`TENANT_ADMIN`

**查询参数**：

- `status`：可选，`enabled` 或 `all`，默认 `all`。组织管理页面默认需要完整树。

**响应**：

```json
{
  "items": [
    {
      "id": 1,
      "code": "01J2X4J1PQ3A8Y8YY0N9V3X7AA",
      "name": "研发中心",
      "path": "/01J2X4J1PQ3A8Y8YY0N9V3X7AA",
      "level": 1,
      "sortOrder": 10,
      "status": "enabled",
      "attributeValue": {
        "valueId": 101,
        "valueCode": "dept_01J2X4J1PQ3A8Y8YY0N9V3X7AA",
        "valueLabel": "研发中心",
        "valuePath": "/01J2X4J1PQ3A8Y8YY0N9V3X7AA",
        "status": "enabled"
      },
      "children": []
    }
  ],
  "total": 1
}
```

## 创建部门

`POST /api/v1/tenant/org-units`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "parentId": 1,
  "name": "数据平台部",
  "sortOrder": 20
}
```

`parentId` 可为空，表示创建根部门。

**响应**：

```json
{
  "orgUnit": {
    "id": 2,
    "code": "01J2X4N6VWZ7HD7R3HPGT9ECY1",
    "name": "数据平台部",
    "path": "/01J2X4J1PQ3A8Y8YY0N9V3X7AA/01J2X4N6VWZ7HD7R3HPGT9ECY1",
    "level": 2,
    "sortOrder": 20,
    "status": "enabled"
  },
  "attributeValue": {
    "valueId": 102,
    "valueCode": "dept_01J2X4N6VWZ7HD7R3HPGT9ECY1",
    "valueLabel": "数据平台部",
    "valuePath": "/01J2X4J1PQ3A8Y8YY0N9V3X7AA/01J2X4N6VWZ7HD7R3HPGT9ECY1"
  }
}
```

**规则**：

- 后端生成 `code/value_code`，前端不得提交。
- 父部门必须属于当前租户且可作为父级。
- 部门和属性值必须同事务创建。

## 编辑部门

`PUT /api/v1/tenant/org-units/:id`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "name": "数据平台与治理部",
  "sortOrder": 30,
  "status": "enabled"
}
```

**响应**：

```json
{
  "orgUnit": {
    "id": 2,
    "code": "01J2X4N6VWZ7HD7R3HPGT9ECY1",
    "name": "数据平台与治理部",
    "sortOrder": 30,
    "status": "enabled"
  }
}
```

**规则**：

- 不允许修改 `code`。
- 改名时同步 `tenant_attribute_values.value_label`。
- 设置 `disabled` 时必须满足停用规则。

## 移动部门

`PUT /api/v1/tenant/org-units/:id/move`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "targetParentId": 10,
  "sortOrder": 40
}
```

`targetParentId` 可为空，表示移动为根部门。

**响应**：

```json
{
  "moved": true,
  "updatedCount": 3
}
```

**规则**：

- 禁止移动到自身或后代下。
- 当前节点及所有后代 `path/level/value_path` 同事务更新。
- `code/value_code` 不变。

## 删除部门

`DELETE /api/v1/tenant/org-units/:id`

**允许角色**：`TENANT_ADMIN`

**响应**：

```json
{
  "deleted": true,
  "id": 2
}
```

**规则**：

- 有子部门或 active 成员时拒绝删除。
- 删除后历史属性解释保留；第一版不做级联硬删除。

## 查询当前租户组织成员

`GET /api/v1/tenant/org-members`

**允许角色**：`TENANT_ADMIN`

**查询参数**：

- `keyword`：按用户名、昵称、邮箱模糊搜索。
- `orgUnitId`：部门筛选。
- `status`：`active`、`inactive` 或 `all`，默认 `active`。
- `page`：页码，默认 1。
- `pageSize`：每页数量，默认 20。

**响应**：

```json
{
  "items": [
    {
      "id": 88,
      "userId": 31,
      "username": "alice",
      "email": "alice@example.com",
      "nickname": "Alice",
      "memberStatus": "active",
      "orgUnit": {
        "id": 2,
        "name": "数据平台部",
        "path": "/01J2X4J1PQ3A8Y8YY0N9V3X7AA/01J2X4N6VWZ7HD7R3HPGT9ECY1"
      },
      "isPrimary": true,
      "positions": ["ORG_LEADER"],
      "systemRoles": ["DO"]
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20
}
```

## 加入部门

`POST /api/v1/tenant/org-members`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "userId": 31,
  "orgUnitId": 2,
  "isPrimary": false
}
```

**响应**：

```json
{
  "member": {
    "id": 88,
    "userId": 31,
    "orgUnitId": 2,
    "isPrimary": true,
    "status": "active"
  }
}
```

**规则**：

- 本接口只负责部门归属，不接收、不修改 `systemRoles`。
- 用户必须是当前租户 active 成员。
- 部门必须属于当前租户且为 enabled。
- 如果这是用户唯一 active 部门，后端自动设为主部门，即使请求传入 `isPrimary=false`。
- 软删除或 inactive 旧记录优先恢复。

## 设置主部门

`PUT /api/v1/tenant/org-members/:id/primary`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "primary": true
}
```

**响应**：

```json
{
  "memberId": 88,
  "userId": 31,
  "primary": true
}
```

**规则**：

- 只能将 active 成员关系设为主部门。
- 同一事务内清除同租户同用户其他 active 成员关系的主部门标记。

## 设置部门职务

`PUT /api/v1/tenant/org-members/:id/positions`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "positions": ["ORG_LEADER"]
}
```

**响应**：

```json
{
  "memberId": 88,
  "positions": ["ORG_LEADER"]
}
```

**规则**：

- 本接口只负责部门职务，不修改系统角色。
- `positions` 只允许 `ORG_LEADER` 和 `DEPUTY_LEADER`。
- 同一部门最多一个 active `ORG_LEADER`。
- 同一部门允许多个 active `DEPUTY_LEADER`。
- 空数组表示移除该成员的特殊职务，该成员仍是普通部门成员。

## 移除部门成员关系

`DELETE /api/v1/tenant/org-members/:id`

**允许角色**：`TENANT_ADMIN`

**请求体**：

```json
{
  "newPrimaryMemberId": 89
}
```

请求体可为空。

**响应**：

```json
{
  "removed": true,
  "memberId": 88,
  "newPrimaryMemberId": 89
}
```

**规则**：

- 删除最后一个部门关系后允许无主部门。
- 删除主部门但仍有其他部门时，优先使用 `newPrimaryMemberId`；未提供时后端在同一事务中明确选择一个剩余部门，或返回错误要求指定。
- 移除成员关系时同事务停用该成员关系下部门职务。

## 系统角色接口

系统角色继续复用现有接口，例如：

`PUT /api/v1/tenants/:id/members/:userId/role`

**规则**：

- 写入 `user_roles`。
- 负责 `DO/DU` 等系统权限。
- 最后一个 `TENANT_ADMIN` 保护继续在现有系统角色服务中实现。
- 组织成员接口不得调用或内联该逻辑。

## 旧组织接口过渡

旧接口：

- `GET /api/v1/tenants/:id/org-units/tree`
- `GET /api/v1/tenants/:id/org-units/:orgUnitId/members`
- `POST /api/v1/tenants/:id/org-units/:orgUnitId/members`
- `PUT /api/v1/tenants/:id/org-units/:orgUnitId/members/:userId/roles`
- `DELETE /api/v1/tenants/:id/org-units/:orgUnitId/members/:userId`

过渡规则：

- 新旧接口必须复用同一个组织管理 Service。
- 旧写接口必须校验路径租户 ID 与后端租户上下文一致。
- 前端组织管理页面不得继续调用旧写接口。
- 前端迁移完成后，旧写接口进入废弃状态。
