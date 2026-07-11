# 数据模型：租户级组织架构与 CP-ABE 属性体系

## 枚举约定

### 状态

- `enabled`：属性、组织单元或属性值可用于新策略构建。
- `disabled`：保留历史记录，但不允许新策略继续选择。
- `active`：成员关系、部门角色或用户属性当前有效。
- `inactive`：关系或属性已失效，仅用于解释历史。

### 属性类型

- `tree`：树形属性，MVP 用于 `department`。
- `enum`：枚举属性，用于 `org_role`、`tenant_role`、`data_category`。
- `number`：数字属性，用于 `security_level`。
- `string`：预留文本属性，MVP 不优先开放。

### 部门角色

- `ORG_MANAGER`：部门主管。
- `ORG_MEMBER`：部门成员。
- `DATA_OWNER`：部门范围内的数据拥有者。
- `DATA_VISITOR`：部门范围内的数据访问者。

## 实体：OrgUnit

**表名**：`tenant_org_units`

**含义**：租户组织架构树节点，是 `department` 属性的真实来源，也是部门角色的作用域。

### 字段

- `id`：主键，后端生成；可返回给前端作为稳定引用。
- `tenant_id`：所属租户；必填；参与所有权限判断。
- `parent_id`：父组织单元；可为空；为空表示根级部门。
- `code`：租户内稳定编码；必填；同租户唯一；策略展示和种子数据使用。
- `name`：中文展示名称；必填；仅用于界面展示和解释。
- `path`：稳定路径；必填；同租户唯一；用于表达“属于某部门及其下级”。
- `level`：层级深度；必填；根节点为 1。
- `sort_order`：同级排序；必填；用于前端组织树展示。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 关系

- 一个租户拥有多个 OrgUnit。
- 一个 OrgUnit 可以有多个子 OrgUnit。
- 一个 OrgUnit 可以有多个 OrgMember。
- 一个 OrgUnit 可以被 `tenant_attribute_values.org_unit_id` 引用。

### 校验规则

- `tenant_id + code` 唯一。
- `tenant_id + path` 唯一。
- `parent_id` 必须属于同一租户。
- 移动节点不得形成循环。
- 停用节点后，新策略不得选择该部门；已保存策略仍可回显并提示状态。

## 实体：OrgMember

**表名**：`tenant_org_members`

**含义**：用户与租户部门的成员关系。

### 字段

- `id`：主键。
- `tenant_id`：所属租户；必填；权限边界字段。
- `org_unit_id`：所属部门；必填；必须属于同一租户。
- `user_id`：用户；必填；必须是当前租户有效成员。
- `status`：`active` 或 `inactive`。
- `source`：来源，MVP 为 `manual` 或 `seed`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 关系

- 一个用户在同一租户可属于多个 OrgUnit。
- 一个 OrgMember 可拥有多个 OrgMemberRole。

### 校验规则

- `tenant_id + org_unit_id + user_id` 唯一。
- 添加成员前必须确认用户存在于 `tenant_users` 且状态有效。
- 移除成员后，该部门范围内的部门角色和用户属性必须失效或重新同步。

## 实体：OrgMemberRole

**表名**：`tenant_org_member_roles`

**含义**：用户在某个部门内拥有的通用角色绑定。

### 字段

- `id`：主键。
- `tenant_id`：所属租户；必填；权限边界字段。
- `org_member_id`：部门成员关系；必填。
- `org_unit_id`：部门作用域；必填；冗余保存用于快速查询。
- `user_id`：用户；必填；冗余保存用于同步用户属性。
- `role_code`：通用部门角色；必填；只允许 `ORG_MANAGER`、`ORG_MEMBER`、`DATA_OWNER`、`DATA_VISITOR`。
- `status`：`active` 或 `inactive`。
- `source`：来源，MVP 为 `manual` 或 `seed`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 关系

- 一个 OrgMember 可以拥有多个通用部门角色。
- 一个用户可在多个部门拥有相同 `role_code`，通过 `org_unit_id` 区分作用域。

### 校验规则

- `tenant_id + org_unit_id + user_id + role_code` 唯一。
- 禁止保存部门专属角色编码，如 `AI_BG_MANAGER`。
- 设置角色必须确认 OrgMember 仍有效。

## 实体：TenantAttribute

**表名**：`tenant_attributes`

**含义**：租户级 CP-ABE 属性定义，决定访问树构建器能选择哪些属性。

### 字段

- `id`：主键。
- `tenant_id`：所属租户；必填。
- `attr_code`：稳定属性编码；必填；如 `department`、`org_role`。
- `attr_name`：展示名称；必填。
- `attr_type`：属性类型；必填；`tree`、`enum`、`number` 或 `string`。
- `value_source`：值来源；如 `org_tree`、`manual`、`derived`。
- `is_required`：是否为用户属性同步的基础属性；默认 false。
- `is_policy_enabled`：是否允许 DATA_OWNER 在策略构建器中选择。
- `description`：业务说明。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 关系

- 一个租户拥有多个 TenantAttribute。
- 一个 TenantAttribute 拥有多个 TenantAttributeValue。
- 一个 UserAttribute 必须引用 TenantAttribute。

### 校验规则

- `tenant_id + attr_code` 唯一。
- `department` 必须是 `tree` 类型，值来源为 `org_tree`。
- `security_level` 必须是 `number` 类型。
- 禁用属性后，新策略不得选择；旧策略可回显并提示。

## 实体：TenantAttributeValue

**表名**：`tenant_attribute_values`

**含义**：租户属性可选值。对于 `department`，值来自组织树；对于枚举属性，值来自租户配置或种子数据。

### 字段

- `id`：主键。
- `tenant_id`：所属租户；必填。
- `attribute_id`：所属属性定义；必填。
- `value_code`：稳定值编码；必填。
- `value_label`：展示名称；必填。
- `value_path`：树形路径或解释路径；可为空。
- `org_unit_id`：当属性为 `department` 时指向 OrgUnit；其他属性为空。
- `sort_order`：排序。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 关系

- 一个 TenantAttributeValue 属于一个 TenantAttribute。
- `department` 类型值一一对应有效 OrgUnit。

### 校验规则

- `tenant_id + attribute_id + value_code` 唯一。
- `department` 值必须引用同租户 OrgUnit。
- enum 值禁用后，新策略不得选择。

## 实体：UserAttribute

**表名**：`user_attributes`

**含义**：用户在某个租户下实际参与 CP-ABE 策略匹配和后续密钥发放的属性集合。

### 字段

- `id`：主键。
- `tenant_id`：所属租户；必填；权限边界字段。
- `user_id`：用户；必填。
- `attribute_id`：属性定义；必填。
- `attr_code`：属性编码冗余；必填；用于快速匹配和解释。
- `value_id`：属性值 ID；树形或枚举属性可填。
- `value_code`：稳定值编码；树形或枚举属性可填。
- `value_label`：展示名称；可填；用于解释。
- `value_path`：树形路径；可填；用于“属于”语义解释。
- `number_value`：数字属性值；数字属性必填。
- `source_type`：来源类型；如 `org_member`、`org_member_role`、`tenant_role`、`manual_seed`。
- `source_id`：来源记录 ID；可为空。
- `status`：`active` 或 `inactive`。
- `synced_at`：最近同步时间。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 关系

- 一个用户在一个租户下拥有多个 UserAttribute。
- UserAttribute 来自组织成员、部门角色、租户角色或演示配置。

### 校验规则

- 用户必须是当前租户有效成员。
- 属性定义必须属于当前租户。
- 树形和枚举属性必须保存 `value_code`；数字属性必须保存 `number_value`。
- 同步失败时不得把部分结果标记为有效。

## 访问树叶子节点模型

### 新结构

```json
{
  "type": "LEAF",
  "attribute": "department",
  "operator": "belongs_to",
  "value": "AI_BG",
  "valueId": 101,
  "valueCode": "AI_BG",
  "label": "AI BG",
  "path": "/AI_BG"
}
```

### 兼容旧结构

```json
{
  "type": "LEAF",
  "attribute": "department",
  "operator": "=",
  "value": "研发部"
}
```

旧结构允许回显；保存时应尽量通过当前租户属性字典补齐 `valueId/valueCode/label/path`。如果无法补齐，后端应返回明确校验错误。

## 状态变更

- OrgUnit：`enabled -> disabled`；禁用后不进入新策略可选值。
- OrgMember：`active -> inactive`；移除成员后同步用户属性。
- OrgMemberRole：`active -> inactive`；角色移除后同步用户属性。
- TenantAttribute：`enabled -> disabled`；禁用后构建器不再展示。
- TenantAttributeValue：`enabled -> disabled`；禁用后构建器不再作为新值选择。
- UserAttribute：同步时旧属性 `active -> inactive`，新属性写入或恢复为 `active`。
