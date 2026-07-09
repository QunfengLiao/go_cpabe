# 接口契约：访问策略管理与访问树构建

## 通用约定

基础路径：`/api/v1`

认证：除登录注册外，本功能所有接口都需要 `Authorization: Bearer <access_token>`。

租户接口：需要 `X-Tenant-Id`，且路径 `:tenantId` 必须与当前租户上下文一致。

响应沿用当前 envelope：

```json
{
  "code": "OK",
  "message": "success",
  "data": {}
}
```

错误响应沿用统一错误 envelope，新增错误码建议见 `plan.md`。

## 数据结构

### PolicyTreeNode

```json
{
  "type": "OR",
  "children": [
    {
      "type": "LEAF",
      "attribute": "role",
      "operator": "=",
      "value": "TENANT_ADMIN"
    }
  ]
}
```

### PolicyAttribute

```json
{
  "id": 1,
  "attrCode": "department",
  "attrName": "部门",
  "attrType": "enum",
  "attrValues": ["研发部", "财务部"],
  "description": "租户成员所属部门",
  "status": "enabled",
  "createdAt": "2026-07-08T10:00:00Z",
  "updatedAt": "2026-07-08T10:00:00Z"
}
```

### PolicyTemplate

```json
{
  "id": 1,
  "name": "数据拥有者或租户管理员可访问",
  "description": "常用租户内管理策略",
  "policyExpr": "role:DATA_OWNER OR role:TENANT_ADMIN",
  "policyTreeJson": {
    "type": "OR",
    "children": []
  },
  "status": "enabled",
  "createdAt": "2026-07-08T10:00:00Z",
  "updatedAt": "2026-07-08T10:00:00Z"
}
```

### AccessPolicy

```json
{
  "id": 1,
  "tenantId": 10,
  "ownerId": 20,
  "name": "研发部数据拥有者访问策略",
  "description": "研发部数据拥有者或租户管理员可访问",
  "policyExpr": "(department:研发部 AND role:DATA_OWNER) OR role:TENANT_ADMIN",
  "policyTreeJson": {
    "type": "OR",
    "children": []
  },
  "status": "enabled",
  "createdAt": "2026-07-08T10:00:00Z",
  "updatedAt": "2026-07-08T10:00:00Z"
}
```

## PLATFORM_ADMIN 属性字典接口

### GET /api/v1/platform/policy-attributes

**权限**：PLATFORM_ADMIN。

**流程**：校验平台管理员 -> 查询未删除属性 -> 按创建时间或编码返回。

**响应**：

```json
{
  "items": [],
  "total": 0
}
```

### POST /api/v1/platform/policy-attributes

**权限**：PLATFORM_ADMIN。

**请求体**：

```json
{
  "attrCode": "department",
  "attrName": "部门",
  "attrType": "enum",
  "attrValues": ["研发部", "财务部"],
  "description": "租户成员所属部门",
  "status": "enabled"
}
```

**流程**：绑定参数 -> 校验编码唯一、类型和值 -> 创建属性。

**错误**：编码重复、类型非法、enum 值为空。

### PUT /api/v1/platform/policy-attributes/:attributeId

**权限**：PLATFORM_ADMIN。

**请求体**：同创建，可局部或完整更新，具体实现阶段统一。

**流程**：查询属性 -> 校验更新值 -> 保存。

**错误**：属性不存在、编码冲突、类型和值非法。

### DELETE /api/v1/platform/policy-attributes/:attributeId

**权限**：PLATFORM_ADMIN。

**流程**：查询属性 -> 软删除或禁用。若已有策略引用该属性，优先软删除并让既有策略在后续校验中提示属性不可用。

## PLATFORM_ADMIN 策略模板接口

### GET /api/v1/platform/policy-templates

**权限**：PLATFORM_ADMIN。

**响应**：

```json
{
  "items": [],
  "total": 0
}
```

### POST /api/v1/platform/policy-templates

**权限**：PLATFORM_ADMIN。

**请求体**：

```json
{
  "name": "指定部门 + 指定角色可访问",
  "description": "部门和角色同时满足",
  "policyTreeJson": {
    "type": "AND",
    "children": []
  },
  "status": "enabled"
}
```

**流程**：校验模板访问树 -> 校验属性启用状态 -> 后端生成 `policyExpr` -> 保存。

### GET /api/v1/platform/policy-templates/:templateId

**权限**：PLATFORM_ADMIN。

**流程**：查询模板详情。

### PUT /api/v1/platform/policy-templates/:templateId

**权限**：PLATFORM_ADMIN。

**流程**：查询模板 -> 校验访问树 -> 重新生成表达式 -> 更新。

### DELETE /api/v1/platform/policy-templates/:templateId

**权限**：PLATFORM_ADMIN。

**流程**：软删除或禁用模板。已从模板创建的访问策略不受模板删除影响。

## DATA_OWNER/TENANT_ADMIN 租户访问策略接口

### GET /api/v1/tenants/:tenantId/access-policy/attributes

**权限**：DATA_OWNER、TENANT_ADMIN 可读；DATA_VISITOR 可按产品需要拒绝，第一阶段建议拒绝。

**流程**：校验租户上下文 -> 返回启用属性字典。

### GET /api/v1/tenants/:tenantId/access-policy/templates

**权限**：DATA_OWNER 可读，TENANT_ADMIN 可读；DATA_VISITOR 第一阶段建议拒绝。

**流程**：校验租户上下文 -> 返回启用模板。

### GET /api/v1/tenants/:tenantId/access-policies

**权限**：DATA_OWNER 查看自己创建的策略；TENANT_ADMIN 查看本租户所有策略。

**查询参数**：`status`、`keyword` 可选。

**流程**：校验租户 -> 根据角色决定查询范围。

### POST /api/v1/tenants/:tenantId/access-policies

**权限**：DATA_OWNER。

**请求体**：

```json
{
  "name": "研发部数据拥有者访问策略",
  "description": "研发部数据拥有者或租户管理员可访问",
  "policyExpr": "(department:研发部 AND role:DATA_OWNER) OR role:TENANT_ADMIN",
  "policyTreeJson": {
    "type": "OR",
    "children": []
  },
  "status": "enabled"
}
```

**流程**：校验租户 -> 校验 DATA_OWNER 角色 -> 校验访问树 -> 生成标准表达式 -> 写入 `tenant_id` 和 `owner_id` -> 保存。

**错误**：无 DATA_OWNER 权限、访问树非法、属性禁用、跨租户。

### GET /api/v1/tenants/:tenantId/access-policies/:policyId

**权限**：DATA_OWNER 只能看自己的策略；TENANT_ADMIN 可看本租户策略。

**流程**：校验租户 -> 按角色查询详情。

### PUT /api/v1/tenants/:tenantId/access-policies/:policyId

**权限**：DATA_OWNER 且必须是 owner。

**流程**：校验租户 -> 按 `tenant_id + owner_id + policy_id` 查找 -> 校验访问树 -> 生成表达式 -> 更新。

### DELETE /api/v1/tenants/:tenantId/access-policies/:policyId

**权限**：DATA_OWNER 且必须是 owner。

**流程**：校验租户 -> 按 `tenant_id + owner_id + policy_id` 查找 -> 软删除。
