# 实现计划：访问策略管理与 DATA_OWNER 可视化访问树构建

**分支**：`feat/access-policy-tree` | **日期**：2026-07-08 | **规格**：[spec.md](./spec.md)

**输入**：来自 `specs/005-access-policy-tree/spec.md` 的功能规格，以及 React + Electron 桌面端补充技术背景。

## 技术方案概览

本功能拆成两条主线推进：

1. PLATFORM_ADMIN 的访问策略管理：平台管理员维护“系统允许使用的策略能力”，包括属性字典、策略模板、启用状态和模板访问树。平台管理员不替 DATA_OWNER 创建具体文件访问策略，也不因此获得租户文件访问或解密权限。
2. DATA_OWNER 的访问策略构建：DATA_OWNER 在租户上下文中基于平台开放的属性和模板，使用 React + Electron Renderer 内的可视化访问树编辑器构建具体访问树，生成 `policy_tree_json` 和 `policy_expr`，并保存为自己的访问策略。

整体技术路线：

- 前端沿用当前 `desktop/` 的 React 19 + Electron 33 + Vite 6 架构和 `HashRouter` 路由，不改 Electron 路由模式。
- 访问树编辑器运行在 Electron Renderer 进程中，业务逻辑放在 React 组件、hooks、service 与纯函数工具中。
- Electron 主进程只负责窗口、应用菜单和受控系统能力，不承载访问树业务逻辑，不直接访问数据库。
- 前端调用后端仍走当前 `request()` HTTP 封装，继续携带 `Authorization` 和 `X-Tenant-Id`。
- 后端沿用 `handler -> service -> repository` 分层，并新增 Policy 相关 domain、repository、service、handler、validator/parser。复杂业务规则不得写在 handler。
- 后端保存前以 `policy_tree_json` 为权威输入重新校验并生成标准 `policy_expr`，不完全信任前端生成的 JSON 或表达式。

## 技术上下文

**语言/版本**：Go 1.23；TypeScript 5.7；React 19；Electron 33；Vite 6。

**主要依赖**：后端 Gin、Gorm、MySQL Driver、Redis Go Client、JWT；前端 React Router、Electron、Vite。访问树画布计划新增 React Flow；自动布局可先用轻量自研层级布局，复杂布局作为增强项评估 dagre 或 elkjs。

**存储**：MySQL 保存属性字典、策略模板和 DATA_OWNER 访问策略；Redis 继续用于既有登录态能力；Renderer 本地草稿优先使用 localStorage，后续可替换为 IndexedDB 或 preload 安全 API。

**测试**：后端 `go test ./...`，重点覆盖访问树校验、表达式生成、权限边界和 handler 路由；前端 `npm run typecheck`、`npm run build`，并补充访问树纯函数单元测试或最小组件交互验证。

**目标平台**：Electron 桌面端 + 本地或演示环境 Go HTTP API。

**项目类型**：桌面应用 + 后端 Web API。

**性能目标**：常见 50 个节点以内访问树编辑保持流畅；保存前校验在用户感知上即时完成；策略列表和详情在演示数据规模下可快速响应。

**约束**：Renderer 不直接使用 Node.js 文件系统能力；前端不得绕过后端直接操作数据库；菜单隐藏只是体验优化，后端必须校验角色、租户和所有者边界；本阶段不实现加密、上传、下载和策略满足性判断。

**范围规模**：新增 3 张核心表，约 16 个 HTTP 接口，约 1 个平台管理页面组、1 个 DATA_OWNER 编辑器页面、1 个列表/详情编辑页面组，以及一组访问树转换和校验工具。

## 宪章检查

*GATE：进入研究前必须通过，Phase 1 设计后已复查。*

- **混合加密边界**：通过。本功能不执行 AES-GCM 文件内容加密、RSA-OAEP、CP-ABE 或 DEK 封装，只生成后续加密链路可使用的访问策略表达。后续文件加密必须仍由 Crypto 模块处理。
- **真实 CP-ABE 实现**：通过。本功能不集成 CP-ABE 库，不允许用访问树或 LSSS 工具冒充真实 CP-ABE 加解密结果。
- **模块边界**：通过。访问树校验、表达式生成、属性字典、模板和访问策略归入 Policy 模块；权限上下文复用认证和租户中间件；Crypto、File、Benchmark、Audit 不在本阶段实现业务逻辑。
- **算法对比口径**：通过。本功能不做 Benchmark，不产生 RSA 与 CP-ABE 性能结论。
- **可解释性**：通过。前端通过访问树画布、表达式预览、JSON 预览、错误高亮解释策略结构；后端错误返回说明非法属性、非法结构、越权和租户边界问题。
- **中文文档**：通过。本功能 SpecKit 产物全部使用简体中文，保留 API 路径、JSON 字段、Go/TypeScript 标识符等必要英文。
- **Go 注释策略**：通过。后续新增 Go 业务代码必须为每个函数和方法写中文前置注释；导出标识符必须符合 GoDoc；实体字段、Handler、Service、Repository、Middleware 注释必须解释业务语义、副作用、权限边界和租户隔离边界。
- **关键注释和可读性检查**：通过。后续 `tasks.md` 必须包含“关键注释和可读性检查”任务，覆盖函数/方法注释、GoDoc 前缀、核心模块业务语义、安全边界和无意义逐行注释清理。

## 项目结构

### 本功能文档

```text
specs/005-access-policy-tree/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── api.md
│   └── frontend-components.md
└── tasks.md
```

### 源码规划

```text
backend/
├── migrations/
│   └── 004_create_policy_tables.sql
└── internal/
    ├── domain/
    │   └── policy.go
    ├── handler/
    │   ├── policy_handler.go
    │   └── policy_handler_test.go
    ├── middleware/
    │   └── tenant.go
    ├── repository/
    │   ├── policy_repository.go
    │   └── tenant_repository.go
    ├── service/
    │   ├── policy_service.go
    │   └── policy_service_test.go
    └── pkg/
        └── policytree/
            ├── tree.go
            ├── validator.go
            ├── expression.go
            └── validator_test.go

desktop/
└── src/
    ├── main/
    │   └── main.ts
    └── renderer/
        └── src/
            ├── api/
            │   └── policy.ts
            ├── components/
            │   └── access-policy/
            │       ├── AccessTreeEditor.tsx
            │       ├── AccessTreeCanvas.tsx
            │       ├── AccessTreeToolbar.tsx
            │       ├── AttributeDictionaryPanel.tsx
            │       ├── PolicyTemplateSelector.tsx
            │       ├── AccessTreeConfigPanel.tsx
            │       ├── PolicyExpressionPreview.tsx
            │       ├── PolicyJsonPreview.tsx
            │       ├── nodes/
            │       │   ├── AndNode.tsx
            │       │   ├── OrNode.tsx
            │       │   └── AttributeNode.tsx
            │       └── tree/
            │           ├── types.ts
            │           ├── convert.ts
            │           ├── expression.ts
            │           ├── validate.ts
            │           └── draft.ts
            └── pages/
                ├── PlatformPolicyManagementPage.tsx
                ├── AccessPolicyBuilderPage.tsx
                ├── MyAccessPoliciesPage.tsx
                └── TenantAccessPolicyViewPage.tsx
```

**结构决策**：沿用当前后端 `domain -> repository -> service -> handler` 分层和当前桌面端 `renderer/src/api`、`renderer/src/components`、`renderer/src/pages` 结构。访问树纯逻辑放入前端 `tree/` 工具目录和后端 `pkg/policytree/`，保证 UI 展示与后端权威校验可分别测试。

## React + Electron 前端架构设计

### 路由与菜单

- 沿用 `HashRouter`，新增路由：
  - `/platform/policies`：PLATFORM_ADMIN 属性字典和策略模板管理。
  - `/access-policies/builder`：DATA_OWNER 新建访问策略。
  - `/access-policies`：DATA_OWNER 我的访问策略。
  - `/access-policies/:policyId/edit`：DATA_OWNER 编辑自己的访问策略。
  - `/tenant/access-policies`：TENANT_ADMIN 查看本租户访问策略。
- 在 `AppLayout` 中把当前“后续模块”的访问策略入口替换为按角色可用的真实菜单。
- 当前代码中后端租户角色常量为 `DO/DU`，需求使用 `DATA_OWNER/DATA_VISITOR`。实现时保留后端现有稳定角色码，前端菜单层提供角色映射：`DO -> DATA_OWNER`，`DU -> DATA_VISITOR`，避免重构既有登录和角色分配。
- 平台路由继续包在 `RequirePlatformAdmin` 下；租户策略路由新增轻量 `RequireTenantRole` 或在页面内根据 `auth.currentTenant.roles` 守卫。无论前端如何守卫，后端仍必须校验。

### API 调用

- 新增 `desktop/src/renderer/src/api/policy.ts`，复用 `request()`。
- 所有租户接口继续通过 `request()` 自动携带 `X-Tenant-Id`，路径中的 `:tenantId` 与当前租户上下文必须一致。
- Renderer 不使用 Node.js 文件系统，不直接调用数据库，不通过 IPC 绕过 Go API。

### 本地草稿缓存

- 第一阶段可不实现草稿缓存。
- 第五阶段使用 `localStorage` 保存“当前用户 + 当前租户 + 策略 ID 或 new”的草稿快照。
- 草稿只保存 UI 编辑状态和访问树内容，不保存 token、密码、私钥或任何密钥材料。
- 打开远端策略时比较 `updated_at` 与草稿 `remoteUpdatedAt`，不一致则提示用户选择继续草稿或丢弃草稿，避免覆盖后端新版本。

## 访问树编辑器组件设计

- **AccessTreeEditor**：编辑器总入口。持有策略基本信息、React Flow `nodes/edges`、选中节点、属性字典、模板、校验错误、dirty、saving。负责调用转换、校验、保存和回显逻辑。
- **AccessTreeCanvas**：React Flow 封装。接收 `nodes`、`edges`、节点类型映射和交互回调；负责拖拽、缩放、选择、高亮、controls、minimap、fitView。
- **AccessTreeToolbar**：顶部工具栏。提供保存、重置、自动布局、居中、预览切换、撤销/重做入口。第一阶段先提供保存、重置、自动布局和居中按钮。
- **AttributeDictionaryPanel**：左侧属性字典。展示平台启用属性、类型、可选值和说明；支持点击或拖拽创建属性叶子节点。
- **PolicyTemplateSelector**：模板选择器。选择模板后将模板 `policy_tree_json` 转为 React Flow 节点与边，作为初始访问树。
- **AccessTreeConfigPanel**：右侧节点配置。根据当前选中节点类型显示逻辑节点类型切换、属性选择、操作符和值输入；enum 使用下拉，string 使用文本，number 使用数字输入。
- **PolicyExpressionPreview**：底部表达式预览。展示由当前访问树生成的表达式；校验失败时标记表达式不可信。
- **PolicyJsonPreview**：底部 JSON 预览。展示格式化后的 `policy_tree_json`，用于演示和调试。
- **AndNode / OrNode**：逻辑节点。视觉上区分 AND 与 OR，显示子节点数量、错误状态、快捷新增/删除操作。
- **AttributeNode**：属性叶子节点。展示属性名称、操作符和值；缺少配置或属性不可用时显示错误边框。

数据流向：

```text
API/模板/草稿 -> policy_tree_json -> treeToFlow -> nodes/edges
用户编辑 nodes/edges -> flowToTree -> policy_tree_json -> validateTree -> policy_expr/错误
保存 -> 后端重新校验 policy_tree_json -> 后端生成标准 policy_expr -> 返回策略详情
```

## 访问树数据结构设计

统一业务 JSON：

```json
{
  "type": "OR",
  "children": [
    {
      "type": "AND",
      "children": [
        {
          "type": "LEAF",
          "attribute": "department",
          "operator": "=",
          "value": "研发部"
        },
        {
          "type": "LEAF",
          "attribute": "role",
          "operator": "=",
          "value": "DATA_OWNER"
        }
      ]
    },
    {
      "type": "LEAF",
      "attribute": "role",
      "operator": "=",
      "value": "TENANT_ADMIN"
    }
  ]
}
```

设计规则：

- `AND` 和 `OR` 必须有 `children`，且至少 2 个子节点。
- `LEAF` 必须有 `attribute`、`operator`、`value`，且不能有子节点。
- 第一阶段操作符只允许 `=` 和 `!=`。
- `value` 在 JSON 中可为字符串或数字；后端根据属性类型校验。
- JSON 不依赖 React Flow 的坐标和 UI 状态，保证后续加密模块不受前端画布实现影响。

## React Flow 与 policy_tree_json 转换方案

### nodes/edges 转 policy_tree_json

1. 从 `nodes` 中找入度为 0 的根节点，必须且只能有一个。
2. 使用 `edges` 构建父子邻接表，按节点的水平位置或显式 `order` 字段稳定排序子节点。
3. 深度优先遍历根节点，生成业务访问树。
4. 遍历时维护 visited 和 recursion stack，发现重复栈节点即判定循环引用。
5. 遍历完成后比较已访问节点数量与 `nodes.length`，发现未访问节点即判定孤立节点。
6. 对每个节点执行类型规则和属性规则校验。

### policy_tree_json 转 nodes/edges

1. 后端返回详情或模板时先校验 JSON 结构。
2. 为每个业务节点生成稳定前端 ID。已有策略可从 DFS 路径生成，例如 `root-0-1`；草稿可保留 UUID。
3. 按树层级生成初始坐标，或交给自动布局函数计算坐标。
4. 为父子关系生成 React Flow edge。
5. 将业务节点的属性、操作符、值放入 `node.data`，将错误状态由校验函数按 nodeId 注入。

### policy_expr 生成

- 前端：用 `policy_tree_json` 生成即时预览，例如 `(department:研发部 AND role:DATA_OWNER) OR role:TENANT_ADMIN`。
- 后端：保存时根据校验通过的 `policy_tree_json` 重新生成标准 `policy_expr`，并以服务端生成结果入库。
- 一致性策略：如果请求体里的 `policyExpr` 与后端生成表达式不同，后端不信任客户端表达式，返回标准表达式；必要时记录为校验提示。

### 为什么不能信任前端 JSON

- Renderer 代码可被调试和篡改，HTTP 请求也可被构造。
- 前端菜单和页面守卫不能证明用户有租户或角色权限。
- 前端可能提交已禁用属性、未开放属性、循环图、多根图或伪造 owner_id。
- 后端必须重新校验租户、角色、所有者、属性启用状态和访问树结构。

## Electron 桌面端适配方案

- 编辑器页面使用全宽工作区，画布区域占据主空间，侧栏和底部预览采用可伸缩布局，避免桌面端大屏只显示窄表单。
- `AccessTreeCanvas` 监听容器尺寸变化，窗口缩放后执行 `fitView` 或保持当前 viewport，避免画布空白或节点溢出。
- 首屏使用 `min-height: calc(100vh - topbar)` 和网格布局：左侧 260-320px，中间自适应，右侧 300-360px，底部可折叠预览。
- Electron 主进程可在第五阶段注册应用菜单或快捷键：
  - `Ctrl+S`：触发保存策略。
  - `Ctrl+Z`：撤销。
  - `Ctrl+Y`：重做。
  - `Ctrl+0`：重置画布缩放。
- 快捷键作为增强项，不阻塞前三阶段交付；实现时优先在 Renderer 捕获编辑器聚焦状态下的键盘事件，避免污染全局应用快捷键。
- 本地草稿缓存仅用于恢复未保存编辑，不作为权威数据源。

## 前端状态管理计划

当前项目未引入 Zustand 或 Redux。第一版使用 React hooks + Context 或编辑器局部 reducer，避免为了单个编辑器引入复杂状态库。

核心状态：

```ts
type AccessTreeEditorState = {
  policyId?: string;
  name: string;
  description: string;
  status: "enabled" | "disabled";
  nodes: FlowNode[];
  edges: FlowEdge[];
  selectedNodeId?: string;
  attributes: PolicyAttribute[];
  templates: PolicyTemplate[];
  policyExpr: string;
  policyTreeJson: PolicyTreeNode | null;
  validationErrors: ValidationError[];
  dirty: boolean;
  saving: boolean;
};
```

状态更新规则：

- 用户修改 `nodes/edges` 后立即标记 `dirty`，触发转换、表达式生成和校验。
- `selectedNodeId` 只影响配置面板和节点高亮，不写入业务 JSON。
- `attributes/templates` 来自 API 或 mock 数据，属性字典变化后重新校验当前树。
- `saving` 只在调用后端保存期间为 true，期间禁用重复提交。
- 草稿缓存保存 `name`、`description`、`status`、`nodes`、`edges`、`selectedNodeId` 和远端版本信息。

## 访问树编辑交互计划

第一阶段必须实现：

- 高质量静态视觉原型。
- mock 属性字典、mock 策略模板、mock 访问树数据。
- 自定义 AND、OR、属性节点展示。
- 节点选中高亮。
- 表达式预览。
- JSON 预览。
- 画布拖动、缩放、居中和基础 controls。

第二阶段实现：

- 新增 AND 节点、新增 OR 节点、新增属性叶子节点。
- 删除节点和边，保持树结构可校验。
- 修改节点类型、属性、操作符和值。
- 生成 `policy_tree_json`。
- 生成 `policy_expr`。
- 基础校验和节点错误高亮。
- 从 `policy_tree_json` 编辑回显。

第三阶段实现：

- 接真实属性字典、策略模板和访问策略接口。
- 保存策略、查询列表、查询详情、编辑策略、删除策略。
- 接入权限控制和菜单控制。

第五阶段增强：

- 本地草稿保存。
- 撤销/重做。
- 快捷键。
- 更细致的错误提示和空状态。
- 自动布局优化。

## 后端架构设计

### 分层职责

- handler 层：绑定路径参数、查询参数和请求体；调用 service；封装统一响应；不做复杂权限和访问树递归逻辑。
- service 层：组织权限校验、租户隔离、所有者边界、属性启用校验、访问树校验、表达式生成、事务边界和业务错误。
- repository 层：封装 `policy_attributes`、`policy_templates`、`access_policies` 的查询和写入；不处理角色业务规则。
- validator/parser 模块：提供访问树结构校验、属性引用校验、表达式生成和 JSON 解析。

### 权限处理

- 平台管理接口使用 `AuthRequired + PlatformAdminRequired`。
- 租户访问策略接口使用 `AuthRequired + TenantRequired`，并在 service 中校验路径 `tenantId` 与中间件租户上下文一致。
- DATA_OWNER 写操作要求当前租户角色包含 `DO` 或兼容映射后的 DATA_OWNER。
- TENANT_ADMIN 只允许列表和详情读取。
- DATA_VISITOR/DU 不允许创建、更新、删除。
- PLATFORM_ADMIN 不允许通过租户内 DATA_OWNER 接口创建具体访问策略。

## 数据库设计

### policy_attributes

用途：平台管理员维护可供 DATA_OWNER 构建访问树的属性字典。

核心字段：

- `id`：主键。
- `attr_code`：属性编码，访问树叶子节点引用该字段。
- `attr_name`：展示名称。
- `attr_type`：`string`、`enum`、`number`。
- `attr_values`：可选值 JSON 或 TEXT，enum 类型必须有值。
- `description`：说明。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`：审计和软删除字段。

索引与约束：

- `uk_policy_attributes_attr_code` 唯一约束，避免同一属性编码歧义。
- `idx_policy_attributes_status` 用于 DATA_OWNER 获取启用属性。
- `idx_policy_attributes_deleted_at` 用于软删除过滤。

### policy_templates

用途：平台管理员维护策略模板，给 DATA_OWNER 提供快速构建入口。

核心字段：

- `id`：主键。
- `name`：模板名称。
- `description`：模板描述。
- `policy_expr`：由模板访问树生成的表达式。
- `policy_tree_json`：模板访问树 JSON。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`。

索引与约束：

- `idx_policy_templates_status`。
- `idx_policy_templates_deleted_at`。
- 可对 `name` 增加普通索引用于管理页搜索。

### access_policies

用途：DATA_OWNER 在租户内保存自己的具体访问策略。

核心字段：

- `id`：主键。
- `tenant_id`：所属租户，参与租户隔离。
- `owner_id`：创建者用户，参与 DATA_OWNER 所有者边界。
- `name`：策略名称。
- `description`：策略描述。
- `policy_expr`：后端根据访问树生成的标准表达式。
- `policy_tree_json`：前端生成、后端校验后的访问树结构。
- `status`：`enabled` 或 `disabled`。
- `created_at`、`updated_at`、`deleted_at`。

索引与约束：

- `idx_access_policies_tenant_owner`：支持 DATA_OWNER 查询自己的策略。
- `idx_access_policies_tenant_status`：支持 TENANT_ADMIN 查看本租户策略。
- `idx_access_policies_deleted_at`。
- 可选唯一约束：`tenant_id + owner_id + name + deleted_at`，防止同一 DATA_OWNER 创建重名活跃策略。

## 接口设计

详细契约见 [contracts/api.md](./contracts/api.md)。实现顺序：

1. 平台属性字典接口：先支持列表和创建，再补更新、删除。
2. 平台策略模板接口：依赖访问树校验和表达式生成，先实现列表和详情，再补创建、更新、删除。
3. DATA_OWNER 可用属性和模板读取接口：复用启用状态过滤。
4. 访问策略列表和详情接口：先支持 DATA_OWNER 自己列表，再支持 TENANT_ADMIN 只读。
5. 访问策略创建、更新、删除接口：接入后端访问树校验、表达式生成、所有者边界。

错误情况统一返回当前响应 envelope，新增错误码建议：

- `POLICY_ATTRIBUTE_CODE_EXISTS`
- `POLICY_ATTRIBUTE_INVALID`
- `POLICY_TEMPLATE_INVALID`
- `ACCESS_POLICY_NOT_FOUND`
- `ACCESS_POLICY_FORBIDDEN`
- `ACCESS_POLICY_TREE_INVALID`
- `ACCESS_POLICY_ATTRIBUTE_DISABLED`
- `ACCESS_POLICY_EXPR_MISMATCH`

## 权限与菜单设计

前端菜单：

- PLATFORM_ADMIN 显示“访问策略管理”。
- DATA_OWNER 显示“访问策略 / 访问策略构建 / 我的访问策略”。
- TENANT_ADMIN 显示“访问策略查看”。
- DATA_VISITOR 不显示访问策略构建菜单。

后端权限：

- PLATFORM_ADMIN 只能管理属性字典和策略模板。
- DATA_OWNER 只能创建、编辑、删除自己的访问策略。
- TENANT_ADMIN 可以查看本租户访问策略列表和详情，不能编辑。
- DATA_VISITOR 不能创建、编辑、删除访问策略。
- 所有 `tenantId` 接口必须校验当前用户有权访问该租户。
- `access_policies.tenant_id` 和 `access_policies.owner_id` 是权限判断的核心字段，不能由客户端信任写入。

## 分阶段实现计划

### 第一阶段：React + Electron 访问树编辑器视觉原型

- 安装并封装 React Flow。
- 使用 mock 属性字典、mock 策略模板和 mock 访问树。
- 完成编辑器三栏加底部预览布局。
- 完成 AND、OR、属性节点视觉设计、选中高亮、错误态样式。
- 完成表达式预览和 JSON 预览。
- 验证桌面窗口缩放、宽屏和全屏体验。

### 第二阶段：访问树编辑能力

- 实现新增、删除、选中、修改节点配置。
- 实现 React Flow 与 `policy_tree_json` 双向转换。
- 实现前端表达式生成和基础校验。
- 实现编辑回显。
- 补充访问树纯函数测试或可运行验证样例。

### 第三阶段：后端 Policy 模块

- 新增数据库迁移和 domain 实体。
- 实现 policy repository。
- 实现 `policytree` 校验和表达式生成。
- 实现 policy service，包含权限、租户、所有者、属性启用状态和访问树校验。
- 实现 handler 和路由注册。
- 补充访问树校验、service 权限和 handler 接口测试。

### 第四阶段：前后端联调

- 前端接真实属性字典和模板接口。
- 平台管理页支持属性和模板 CRUD。
- DATA_OWNER 页面支持保存、列表、详情、编辑、删除。
- TENANT_ADMIN 页面支持只读查看。
- 接入菜单和路由守卫。
- 执行端到端手工验收脚本。

### 第五阶段：Electron 桌面体验优化

- 本地草稿缓存和冲突提示。
- 快捷键和应用菜单增强。
- 撤销/重做。
- 更细致的空状态、错误提示、自动布局和画布自适应。
- 性能检查和样式打磨，避免退化为普通 CRUD 页面。

## 风险与规避方案

- React Flow `nodes/edges` 与业务 JSON 不一致：将业务 JSON 作为保存前权威结构，所有保存都先执行 `flowToTree` 与校验；后端再次校验。
- 用户拖拽节点后无法稳定生成树结构：边关系决定树结构，坐标只决定展示和子节点排序，不作为父子关系来源。
- 孤立节点、多根节点、循环节点：转换函数统一检查入度、遍历覆盖率和递归栈。
- `policy_expr` 与 `policy_tree_json` 不一致：后端保存时重新生成标准表达式并覆盖客户端表达式。
- 前端生成的访问树不可信：后端校验结构、属性启用状态、租户角色和所有者边界。
- Electron Renderer 误用 Node.js 能力：Renderer 不引入 `fs` 等 Node API；草稿优先 localStorage；如需系统能力通过 preload 暴露最小 API。
- 本地草稿与真实后端数据冲突：草稿记录远端 `updated_at`，打开时检测冲突并让用户选择。
- DATA_OWNER 越权修改他人策略：更新和删除时 repository 按 `tenant_id + owner_id + policy_id` 查询，找不到即拒绝。
- 普通用户跨租户访问策略：租户接口使用 `TenantRequired` 并校验路径 tenantId 与上下文一致。
- 一次性做太复杂导致不可控：按五阶段推进，前三阶段先闭合核心链路，快捷键、草稿和复杂布局放到增强阶段。
- 页面做成普通 CRUD、缺少作品感：第一阶段先把访问树编辑器作为主体验打磨，平台 CRUD 页面保持简洁，DATA_OWNER 构建页面保持画布优先。

## 测试计划

- 后端单元测试：
  - 合法 AND/OR/LEAF 树通过校验。
  - 空根、多根、孤立节点、循环引用、AND/OR 子节点不足被拒绝。
  - enum/string/number 属性值校验。
  - 禁用属性和未开放属性被拒绝。
  - 表达式生成稳定且带必要括号。
- 后端 service 测试：
  - PLATFORM_ADMIN 只能管理属性和模板。
  - DATA_OWNER 只能写自己的策略。
  - TENANT_ADMIN 只能读本租户策略。
  - DATA_VISITOR 写操作被拒绝。
  - 跨租户访问被拒绝。
- 后端 handler 测试：
  - 平台属性接口和模板接口基本成功/失败路径。
  - 租户访问策略创建、列表、详情、更新、删除权限路径。
- 前端类型与构建：
  - `npm run typecheck`
  - `npm run build`
- 前端访问树工具测试：
  - `treeToFlow` 和 `flowToTree` 双向转换。
  - `generatePolicyExpr` 表达式生成。
  - `validateTree` 错误定位。
- 手工验收：
  - 按 PLATFORM_ADMIN、DATA_OWNER、TENANT_ADMIN、DATA_VISITOR 四类账号验证菜单与受保护操作。
  - 在 Electron 桌面窗口中验证拖拽、缩放、居中、保存、回显和错误高亮。

## 非目标确认

本阶段不做文件上传、AES 加密、CP-ABE 加密、密钥生成、用户私钥分发、文件下载、策略满足性判断、完整 RBAC 后台，也不重构已完成的登录、多租户、租户成员和角色分配功能。

## Phase 1 设计后宪章复查

- 设计产物未引入加密实现，因此不违反混合加密和真实 CP-ABE 原则。
- Policy 相关职责已独立规划，未把密码算法放入 Handler、Service 或前端编辑器。
- 文档、契约、数据模型和 quickstart 均使用简体中文。
- 后续 tasks 必须显式包含 Go 注释和可读性检查任务。

## Agent 上下文更新

仓库当前仅提供 `.specify/scripts/powershell/setup-plan.ps1`、`setup-tasks.ps1`、`check-prerequisites.ps1` 等脚本，未提供自动更新 agent 上下文的脚本。因此本阶段未执行自动上下文更新。当前 `AGENTS.md` 已包含项目宪章、技术栈、模块边界、密码学约束和中文文档规范，后续任务可直接依据本计划继续。

## 复杂度跟踪

无宪章违规项，不需要复杂度例外。
