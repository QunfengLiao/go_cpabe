# 快速验证：租户组织架构管理

## 前置条件

- 后端数据库已包含 `006-tenant-org-attributes` 的组织与属性基础表。
- 当前功能迁移 `008_tenant_org_management.sql` 已执行。
- 至少存在一个租户，且有一个 active `TENANT_ADMIN`。
- 桌面端能以该租户管理员身份登录并选择当前租户。

## 验证 1：旧数据统计与迁移

1. 执行旧部门职务统计：

```sql
SELECT role_code, status, COUNT(*) AS total
FROM tenant_org_member_roles
GROUP BY role_code, status
ORDER BY role_code, status;
```

2. 执行迁移脚本。
3. 重复执行迁移脚本两次。
4. 再次统计 `tenant_org_member_roles` 和 `user_roles`。

**期望结果**：

- `ORG_MANAGER` 已迁移为 `ORG_LEADER`。
- `ORG_MEMBER`、`DATA_OWNER`、`DATA_VISITOR` 不再作为 active 部门职务存在。
- `DATA_OWNER` 对应用户拥有或保留 `DO`。
- `DATA_VISITOR` 对应用户拥有或保留 `DU`。
- 重复执行不产生重复授权或重复职务。

## 验证 2：部门树维护

1. 以当前租户 `TENANT_ADMIN` 登录。
2. 调用 `GET /api/v1/tenant/org-units/tree`。
3. 创建一个根部门。
4. 在根部门下创建一个子部门。
5. 修改子部门名称。
6. 移动子部门到另一个父部门。

**期望结果**：

- 所有返回数据只属于当前租户。
- 部门创建后自动生成 `code` 和属性 `value_code`。
- 改名只改变 `name/value_label`。
- 移动只改变 `path/level/value_path`。
- `code/value_code` 始终不变。

## 验证 3：部门停用与删除保护

1. 对存在 enabled 子部门的父部门执行停用。
2. 对存在 active 成员的部门执行删除。
3. 对无子部门且无 active 成员的部门执行删除。

**期望结果**：

- 第一种操作被拒绝。
- 第二种操作被拒绝。
- 第三种操作成功。
- 停用部门不再可新增成员，也不再出现在新策略可选部门中。

## 验证 4：成员归属与主部门

1. 选择当前租户 active 用户。
2. 将该用户加入第一个部门。
3. 将该用户加入第二个部门。
4. 将第二个部门设为主部门。
5. 删除主部门关系。
6. 删除最后一个部门关系。

**期望结果**：

- 第一个部门自动成为主部门。
- 加入第二个部门后仍只有一个主部门。
- 设置主部门后旧主部门被清除。
- 删除主部门时后端指定或选择新的主部门。
- 删除最后一个部门后允许无主部门。

## 验证 5：部门职务与系统角色分离

1. 为某部门成员设置 `ORG_LEADER`。
2. 尝试为同部门另一成员设置 `ORG_LEADER`。
3. 为多个成员设置 `DEPUTY_LEADER`。
4. 尝试通过部门职务接口提交 `DO`、`DU`、`TENANT_ADMIN`、`DATA_OWNER`、`DATA_VISITOR` 或 `ORG_MEMBER`。
5. 通过现有成员角色接口修改用户系统角色。

**期望结果**：

- 同部门最多一个负责人。
- 多个副负责人可保存。
- 非法职务编码全部被拒绝。
- 系统角色只写入 `user_roles`，不写入 `tenant_org_member_roles`。

## 验证 6：前端组织管理页面

1. 打开桌面端。
2. 使用 `TENANT_ADMIN` 进入当前租户。
3. 在左侧菜单进入“组织管理”。
4. 在“组织架构”页签创建、编辑、移动部门。
5. 在“成员管理”页签搜索成员，调整部门、主部门、部门职务和系统角色。

**期望结果**：

- 页面包含“组织架构”和“成员管理”两个页签。
- 创建和编辑使用抽屉。
- 危险操作使用设计系统确认组件，不出现浏览器原生 `confirm`。
- 同一个成员抽屉中，部门职务和系统角色分别调用各自接口。

## 验证命令

后端：

```bash
cd backend
go test ./...
```

前端：

```bash
cd desktop
npm run typecheck
npm run test
```

## 本轮实现验证记录

- 旧部门职务统计：此前开发库只读聚合查询未返回 `tenant_org_member_roles` 旧数据；其他环境执行 `008_tenant_org_management.sql` 前仍必须重新统计。
- 迁移脚本：已新增 `backend/migrations/008_tenant_org_management.sql`，包含 `is_primary` 字段、旧 `role_code` 迁移、`DO/DU` 补齐、旧职务停用和主部门归一。
- 前端验证：已执行 `npm run typecheck`，通过；已执行 `npm run test`，6 个测试文件、20 个用例通过。
- 后端验证：当前执行环境未安装 `go`/`gofmt`，无法在本机运行 `gofmt` 与 `go test ./...`；需要在具备 Go 工具链的环境补跑。
