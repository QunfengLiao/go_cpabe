# 数据模型：访问策略管理与 DATA_OWNER 可视化访问树构建

## 枚举和值对象

### PolicyStatus

- `enabled`：启用，可被 DATA_OWNER 使用或展示。
- `disabled`：停用，不再作为新策略构建入口，但历史数据保留。

### PolicyAttributeType

- `string`：属性值由 DATA_OWNER 输入文本。
- `enum`：属性值必须来自平台维护的可选值。
- `number`：属性值必须是数字。

### PolicyOperator

- `=`：等于。
- `!=`：不等于。

后续预留 `>`、`>=`、`<`、`<=`，本阶段不作为必做。

## 实体：PolicyAttribute

**业务含义**：平台管理员维护的策略属性字典，限制 DATA_OWNER 构建访问树时能引用哪些属性。

| 字段 | 类型建议 | 约束 | 说明 |
|------|----------|------|------|
| `id` | uint64 | 主键 | 属性字典唯一标识 |
| `attr_code` | string | 必填、唯一、软删除后仍需避免活跃重复 | 访问树叶子节点引用的稳定编码 |
| `attr_name` | string | 必填 | 展示名称 |
| `attr_type` | string | 必填，枚举 `string/enum/number` | 决定前端输入控件和后端值校验 |
| `attr_values` | JSON/TEXT | enum 必填，其他类型可空 | enum 可选值列表 |
| `description` | string | 可空 | 管理说明 |
| `status` | string | 必填，默认 `enabled` | 是否开放给 DATA_OWNER 使用 |
| `created_at` | time | 自动写入 | 创建时间 |
| `updated_at` | time | 自动更新 | 更新时间 |
| `deleted_at` | soft delete | 可空 | 软删除标记 |

**索引**：

- `uk_policy_attributes_attr_code`
- `idx_policy_attributes_status`
- `idx_policy_attributes_deleted_at`

**校验规则**：

- `attr_code` 只能使用稳定编码，建议小写字母、数字和下划线。
- `attr_type=enum` 时 `attr_values` 必须是非空数组。
- 禁用属性不能作为新访问树叶子节点使用。

## 实体：PolicyTemplate

**业务含义**：平台管理员维护的策略模板，为 DATA_OWNER 提供访问树起点。模板不是具体租户授权。

| 字段 | 类型建议 | 约束 | 说明 |
|------|----------|------|------|
| `id` | uint64 | 主键 | 模板唯一标识 |
| `name` | string | 必填 | 模板名称 |
| `description` | string | 可空 | 模板说明 |
| `policy_expr` | text | 必填，由访问树生成 | 可读表达式 |
| `policy_tree_json` | JSON/TEXT | 必填 | 模板访问树结构 |
| `status` | string | 必填，默认 `enabled` | 是否可作为 DATA_OWNER 新策略起点 |
| `created_at` | time | 自动写入 | 创建时间 |
| `updated_at` | time | 自动更新 | 更新时间 |
| `deleted_at` | soft delete | 可空 | 软删除标记 |

**索引**：

- `idx_policy_templates_status`
- `idx_policy_templates_deleted_at`
- `idx_policy_templates_name`

**校验规则**：

- 创建或更新模板时必须校验 `policy_tree_json`。
- 模板引用的属性必须存在且启用。
- 保存时由后端生成标准 `policy_expr`。

## 实体：AccessPolicy

**业务含义**：DATA_OWNER 在租户内创建的具体访问策略，后续文件上传和 CP-ABE 加密会引用该策略。

| 字段 | 类型建议 | 约束 | 说明 |
|------|----------|------|------|
| `id` | uint64 | 主键 | 访问策略唯一标识 |
| `tenant_id` | uint64 | 必填、索引 | 所属租户，参与租户隔离 |
| `owner_id` | uint64 | 必填、索引 | 创建者用户，参与 DATA_OWNER 所有者边界 |
| `name` | string | 必填 | 策略名称 |
| `description` | string | 可空 | 策略说明 |
| `policy_expr` | text | 必填，由访问树生成 | 标准表达式 |
| `policy_tree_json` | JSON/TEXT | 必填 | 后端校验通过后的访问树结构 |
| `status` | string | 必填，默认 `enabled` | 是否可用于后续文件策略选择 |
| `created_at` | time | 自动写入 | 创建时间 |
| `updated_at` | time | 自动更新 | 更新时间 |
| `deleted_at` | soft delete | 可空 | 软删除标记 |

**索引**：

- `idx_access_policies_tenant_owner`
- `idx_access_policies_tenant_status`
- `idx_access_policies_deleted_at`
- 可选：`uk_access_policies_tenant_owner_name_active`

**校验规则**：

- DATA_OWNER 创建策略时，`tenant_id` 和 `owner_id` 必须由后端上下文写入，不信任客户端。
- 更新和删除时必须匹配 `tenant_id + owner_id + policy_id`。
- TENANT_ADMIN 只能按 `tenant_id` 查询列表和详情。

## 值对象：PolicyTreeNode

### LogicNode

```json
{
  "type": "AND",
  "children": []
}
```

字段：

- `type`：`AND` 或 `OR`。
- `children`：至少两个子节点。

### LeafNode

```json
{
  "type": "LEAF",
  "attribute": "department",
  "operator": "=",
  "value": "研发部"
}
```

字段：

- `type`：固定为 `LEAF`。
- `attribute`：引用 `policy_attributes.attr_code`。
- `operator`：本阶段为 `=` 或 `!=`。
- `value`：按属性类型校验。

## 状态流转

### PolicyAttribute

```text
创建 enabled -> disabled -> enabled
创建 enabled/disabled -> soft deleted
```

### PolicyTemplate

```text
创建 enabled -> disabled -> enabled
创建 enabled/disabled -> soft deleted
```

### AccessPolicy

```text
DATA_OWNER 创建 enabled -> disabled -> enabled
DATA_OWNER 创建 enabled/disabled -> soft deleted
```

## 关系

- `PolicyTemplate.policy_tree_json` 引用多个 `PolicyAttribute.attr_code`，但不建立数据库外键。
- `AccessPolicy.policy_tree_json` 引用多个 `PolicyAttribute.attr_code`，保存时由后端校验属性存在且启用。
- `AccessPolicy.tenant_id` 对应租户表主键。
- `AccessPolicy.owner_id` 对应用户表主键。

## 数据安全边界

- `tenant_id` 和 `owner_id` 不允许客户端指定为权威值。
- `policy_tree_json` 由前端生成但必须由后端校验。
- `policy_expr` 由后端生成后入库。
- 本阶段不存储密钥、明文文件、密文文件或用户私钥。
