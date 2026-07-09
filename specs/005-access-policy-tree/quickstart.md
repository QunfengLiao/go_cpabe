# 快速验证指南：访问策略管理与 DATA_OWNER 可视化访问树构建

## 前置条件

- 已完成登录、多租户、租户成员和角色分配能力。
- 至少存在四类测试账号或可切换角色：PLATFORM_ADMIN、TENANT_ADMIN、DATA_OWNER、DATA_VISITOR。当前代码内部角色若使用 `DO/DU`，测试时按 `DO=DATA_OWNER`、`DU=DATA_VISITOR` 映射验证。
- 后端环境变量已配置 MySQL 和 Redis。

## 准备数据库

```powershell
cd D:\04_code\go_cpabe\backend
mysql < migrations/004_create_policy_tables.sql
```

预期结果：

- 创建 `policy_attributes`。
- 创建 `policy_templates`。
- 创建 `access_policies`。

## 启动后端

```powershell
cd D:\04_code\go_cpabe\backend
go test ./...
go run ./cmd/server
```

预期结果：

- 测试通过。
- 后端监听本地 API 地址。
- `/health` 返回 app、mysql、redis 状态。

## 启动桌面端

```powershell
cd D:\04_code\go_cpabe\desktop
npm run typecheck
npm run build
npm run dev:electron:sangfor
```

预期结果：

- TypeScript 检查通过。
- Electron 应用启动。
- 仍沿用 HashRouter 和现有租户启动逻辑。

## 验证 PLATFORM_ADMIN 属性和模板管理

1. 使用 PLATFORM_ADMIN 登录。
2. 确认侧边栏显示“访问策略管理”。
3. 创建属性：
   - `role`，类型 `enum`，值包含 `DATA_OWNER`、`TENANT_ADMIN`。
   - `department`，类型 `enum`，值包含 `研发部`。
4. 创建策略模板：`数据拥有者或租户管理员可访问`。
5. 确认模板保存后系统返回标准 `policyExpr`。

预期结果：

- PLATFORM_ADMIN 可以管理属性和模板。
- PLATFORM_ADMIN 看不到 DATA_OWNER 的“访问策略构建”入口。
- PLATFORM_ADMIN 无法通过租户内访问策略接口创建具体策略。

## 验证 DATA_OWNER 可视化构建

1. 使用 DATA_OWNER 登录并进入目标租户。
2. 确认侧边栏显示：
   - 访问策略构建
   - 我的访问策略
3. 进入访问策略构建页面。
4. 从模板选择器应用“数据拥有者或租户管理员可访问”。
5. 在画布中新增或修改 AND、OR、属性叶子节点。
6. 查看表达式预览和 JSON 预览。
7. 保存策略。
8. 进入“我的访问策略”，打开刚保存的策略详情或编辑页。

预期结果：

- 访问树能在画布中展示。
- `policy_tree_json` 和 `policy_expr` 实时更新。
- 保存成功后重新打开能完整回显。

## 验证校验失败

1. 创建一个只有一个子节点的 AND 节点。
2. 创建一个缺少属性值的 LEAF 节点。
3. 尝试保存。

预期结果：

- 前端阻止保存或提示错误。
- 对应节点高亮。
- 若绕过前端直接提交，后端仍返回明确错误。

## 验证 TENANT_ADMIN 只读

1. 使用 TENANT_ADMIN 登录同一租户。
2. 进入“访问策略查看”。
3. 查看 DATA_OWNER 创建的策略列表和详情。
4. 尝试编辑或删除。

预期结果：

- 列表和详情可查看。
- 编辑和删除入口不可用。
- 直接调用写接口被后端拒绝。

## 验证 DATA_VISITOR 禁止访问

1. 使用 DATA_VISITOR 登录。
2. 查看侧边栏。
3. 尝试访问访问策略构建路径。

预期结果：

- 不显示访问策略构建菜单。
- 页面守卫阻止进入或展示无权限。
- 后端写接口返回权限错误。

## 验证跨租户和所有者边界

1. DATA_OWNER A 在租户 1 创建策略。
2. DATA_OWNER B 尝试编辑 A 的策略。
3. DATA_OWNER A 切换到租户 2 或伪造租户 ID 尝试访问租户 1 策略。

预期结果：

- B 无法编辑或删除 A 的策略。
- 跨租户访问被拒绝。
- 后端错误信息明确说明权限或租户边界问题。

## 非目标确认

执行本指南时不验证文件上传、AES 加密、CP-ABE 加密、密钥生成、用户私钥分发、文件下载、策略满足性判断和完整 RBAC 后台。

## 2026-07-08 本轮执行记录

- 已执行 `desktop` 前端验证：`npm.cmd run test` 通过，3 个测试文件、6 个测试用例全部通过。
- 已执行 `desktop` 类型检查：`npm.cmd run typecheck` 通过。
- 已执行 `desktop` 构建：`npm.cmd run build` 通过；Vite 报告单个产物 chunk 超过 500 kB，为 React Flow 引入后的体积提示，不影响构建结果。
- 未能执行 `backend` 的 `gofmt`、`go test ./...`、`go run ./cmd/server` 和依赖后端启动的四角色手工验收，因为当前环境中 `go` 和 `gofmt` 不在 PATH，`where.exe go` 与 `where.exe gofmt` 均未找到可执行文件。
- 待 Go 工具链可用后，需要重新执行本指南中的“启动后端”和四类角色验收步骤，并把实际接口、菜单和权限边界结果补记到本节。
