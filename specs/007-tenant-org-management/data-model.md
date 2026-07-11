# 数据模型：租户组织架构管理

## 枚举约定

### 部门状态

- `enabled`：部门可用于新增成员和新策略选择。
- `disabled`：部门停用，历史策略和历史密文仍可解释，但不能新增成员或被新策略选择。

### 成员关系状态

- `active`：用户当前属于该部门。
- `inactive`：成员关系已移除或失效，仅保留历史解释。

### 部门职务

- `ORG_LEADER`：部门负责人。每个部门最多一个 active 负责人。
- `DEPUTY_LEADER`：部门副负责人。每个部门可有多个 active 副负责人。

以下编码不得再写入 `tenant_org_member_roles`：

- `MEMBER`
- `ORG_MEMBER`
- `ORG_MANAGER`
- `DATA_OWNER`
- `DATA_VISITOR`
- `TENANT_ADMIN`
- `DO`
- `DU`
- `PLATFORM_ADMIN`

普通成员身份由 `tenant_org_members` active 记录自然表达。

## 实体：TenantOrgUnit

**表名**：`tenant_org_units`

**业务含义**：当前租户部门树节点，是成员归属和 `department` 属性值的来源。

### 字段

- `id`：数据库主键，可作为前端操作部门的记录 ID。
- `tenant_id`：所属租户，必须来自后端租户上下文，参与所有查询和写入边界。
- `parent_id`：父部门 ID，可为空；为空表示根部门。
- `code`：不可变稳定编码，创建时使用 ULID 或 UUID 生成；同一租户内唯一，创建后禁止修改。
- `name`：部门展示名称，可修改；用于页面展示和属性 `value_label`。
- `path`：部门解释路径，基于稳定 `code` 组成；移动部门时更新，只用于展示和“属于”解释，不作为策略权威判断。
- `level`：部门层级；根部门为 1，移动部门时当前节点及后代同步更新。
- `sort_order`：同级排序；只影响展示顺序。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 约束

- `tenant_id + code` 唯一。
- `tenant_id + path` 唯一。
- `parent_id` 必须属于同一租户。
- 移动时不得形成循环。
- 存在 enabled 子部门时第一版禁止停用父部门。
- 有子部门或 active 成员时禁止直接删除。

### 属性同步

- 创建部门时创建对应 `tenant_attribute_values`。
- 改名时同步 `value_label`。
- 移动时同步 `value_path`。
- 停用时同步属性值状态。
- `code/value_code` 永不因改名或移动而改变。

## 实体：TenantAttributeValue

**表名**：`tenant_attribute_values`

**业务含义**：部门对应的 CP-ABE 属性可选值。`department` 类型属性值与部门一一对应。

### 字段

- `id`：属性值主键，访问策略可引用它作为稳定值 ID。
- `tenant_id`：所属租户，必须与部门一致。
- `attribute_id`：所属属性定义，部门值必须指向当前租户 `department` 属性。
- `value_code`：基于部门不可变 `code` 生成的稳定属性值编码，创建后禁止修改。
- `value_label`：展示名称，部门改名时同步。
- `value_path`：解释路径，部门移动时同步。
- `org_unit_id`：关联部门 ID，部门属性值必填。
- `sort_order`：展示排序。
- `status`：`enabled` 或 `disabled`，部门停用时同步。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 约束

- `tenant_id + attribute_id + value_code` 唯一。
- `org_unit_id` 必须属于同一租户。
- 访问策略必须引用 `id` 或 `value_code`，不得依赖 `value_path`。

## 实体：TenantOrgMember

**表名**：`tenant_org_members`

**业务含义**：用户与部门的归属关系。该表本身表达普通部门成员身份。

### 字段

- `id`：成员关系主键，成员管理接口使用该 ID 调整主部门、职务或移除关系。
- `tenant_id`：所属租户，必须来自后端租户上下文。
- `org_unit_id`：所属部门，必须属于同一租户且状态允许新增成员。
- `user_id`：用户 ID，必须是 `tenant_users` 当前租户 active 成员。
- `is_primary`：是否主部门；同一租户同一用户在多个 active 部门中必须且只能有一个主部门。
- `status`：`active` 或 `inactive`。
- `source`：来源，例如 `manual` 或 `seed`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 约束

- `tenant_id + org_unit_id + user_id` 唯一。
- inactive 或软删除旧记录再次添加时优先恢复。
- active 部门数为 0：允许无主部门。
- active 部门数为 1：该关系自动为主部门。
- active 部门数大于 1：必须且只能有一个主部门。
- 删除主部门且仍有其他 active 部门时，必须同事务指定或选择新主部门。

## 实体：TenantOrgMemberRole

**表名**：`tenant_org_member_roles`

**业务含义**：用户在某个部门成员关系上的特殊组织职务，只表达负责人或副负责人。

### 字段

- `id`：职务记录主键。
- `tenant_id`：所属租户，必须与成员关系和部门一致。
- `org_member_id`：所属部门成员关系，必须 active 才能新增 active 职务。
- `org_unit_id`：冗余部门 ID，用于快速校验每部门负责人唯一。
- `user_id`：冗余用户 ID，用于成员列表聚合。
- `role_code`：只允许 `ORG_LEADER` 或 `DEPUTY_LEADER`。
- `status`：`active` 或 `inactive`。
- `source`：来源，例如 `manual` 或 `seed`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

### 约束

- 同一 `tenant_id + org_unit_id + user_id + role_code` 不产生重复 active 记录。
- 同一 `tenant_id + org_unit_id` 最多一个 active `ORG_LEADER`。
- 同一部门允许多个 active `DEPUTY_LEADER`。
- `ORG_MEMBER`、`MEMBER`、`DATA_OWNER`、`DATA_VISITOR` 和系统角色不得写入本表。

## 实体：UserRoleAssignment

**表名**：`user_roles`

**业务含义**：系统权限角色授权，继续保存 `TENANT_ADMIN`、`DO`、`DU` 等租户级权限。

### 本功能关联规则

- 部门成员接口不得写入该表。
- 部门职务接口不得写入该表。
- 系统角色继续由现有成员角色接口维护。
- 旧 `DATA_OWNER` 部门职务迁移时只用于补齐或保留 `DO`。
- 旧 `DATA_VISITOR` 部门职务迁移时只用于补齐或保留 `DU`。
- 最后一个 `TENANT_ADMIN` 保护继续由现有系统角色服务负责。

## 状态变更

```text
部门：enabled -> disabled
成员关系：active -> inactive -> active（恢复旧记录）
部门职务：active -> inactive -> active（恢复旧记录）
属性值：enabled -> disabled
```

删除部门采用软删除或拒绝删除；第一版不做级联硬删除。

## 旧数据迁移映射

| 旧 role_code | 迁移动作 | 说明 |
| --- | --- | --- |
| `ORG_MANAGER` | 映射为 `ORG_LEADER` | 若同部门出现多个负责人冲突，迁移应失败并要求人工处理 |
| `ORG_MEMBER` | 停用或删除 | 普通成员由 `tenant_org_members` 表达，不迁移成 `MEMBER` |
| `DATA_OWNER` | 先补齐 `user_roles.DO`，再停用旧职务 | 不迁移为普通成员或部门职务 |
| `DATA_VISITOR` | 先补齐 `user_roles.DU`，再停用旧职务 | 不迁移为普通成员或部门职务 |

当前开发库统计结果：执行只读聚合查询时未返回旧 `tenant_org_member_roles` 数据。其他环境仍必须在迁移前重新统计。
