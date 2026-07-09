# 前端组件契约：访问树编辑器

## AccessTreeEditor

**职责**：访问树编辑器总入口，组织策略基本信息、属性字典、模板、画布状态、预览、校验、保存和回显。

**输入**：

- `mode`: `create | edit`
- `policyId?`: 编辑模式下的策略 ID
- `tenantId`: 当前租户 ID

**输出/回调**：

- `onSaved(policy)`：保存成功后返回标准策略。
- `onCancel()`：取消编辑。

**状态来源**：

- 创建模式加载可用属性和模板。
- 编辑模式加载策略详情后执行 `treeToFlow`。

## AccessTreeCanvas

**职责**：封装 React Flow 画布，负责节点、边、拖拽、缩放、选中、高亮和画布控制。

**输入**：

- `nodes`
- `edges`
- `validationErrors`
- `selectedNodeId`

**回调**：

- `onNodesChange`
- `onEdgesChange`
- `onSelectNode`
- `onConnect`
- `onDeleteNode`

## AccessTreeToolbar

**职责**：提供保存、重置、自动布局、居中、撤销、重做和预览切换入口。

**输入**：

- `dirty`
- `saving`
- `canUndo`
- `canRedo`
- `hasValidationError`

**回调**：

- `onSave`
- `onReset`
- `onAutoLayout`
- `onFitView`
- `onUndo`
- `onRedo`

## AttributeDictionaryPanel

**职责**：展示平台开放属性，并作为创建属性叶子节点的入口。

**输入**：

- `attributes`
- `selectedAttributeCode?`

**回调**：

- `onCreateAttributeNode(attributeCode)`
- `onSelectAttribute(attributeCode)`

## PolicyTemplateSelector

**职责**：选择平台启用模板，并将模板访问树作为当前编辑器起点。

**输入**：

- `templates`
- `currentTemplateId?`
- `dirty`

**回调**：

- `onApplyTemplate(template)`

**交互约束**：当编辑器已有未保存修改时，应用模板前必须提示会覆盖当前画布。

## AccessTreeConfigPanel

**职责**：编辑当前选中节点。

**输入**：

- `selectedNode`
- `attributes`
- `validationErrors`

**回调**：

- `onChangeNodeType(type)`
- `onChangeAttribute(attributeCode)`
- `onChangeOperator(operator)`
- `onChangeValue(value)`
- `onDeleteNode(nodeId)`

## PolicyExpressionPreview

**职责**：展示当前访问树表达式。

**输入**：

- `policyExpr`
- `validationErrors`

**规则**：当存在结构性错误时展示“表达式待修正”，避免用户误认为非法表达式可保存。

## PolicyJsonPreview

**职责**：展示格式化后的 `policy_tree_json`。

**输入**：

- `policyTreeJson`
- `validationErrors`

## AndNode / OrNode / AttributeNode

**职责**：React Flow 自定义节点。

**共同输入**：

- `selected`
- `hasError`
- `errorMessage?`

**AndNode/OrNode 额外展示**：

- 子节点数量。
- 快捷新增子节点入口。

**AttributeNode 额外展示**：

- 属性名称。
- 操作符。
- 属性值。

## 前端纯函数契约

### flowToTree(nodes, edges)

**输入**：React Flow nodes/edges。

**输出**：

```ts
{
  tree: PolicyTreeNode | null;
  errors: ValidationError[];
}
```

**规则**：检查单根、无循环、无孤立节点，并输出业务树。

### treeToFlow(tree)

**输入**：`PolicyTreeNode`。

**输出**：

```ts
{
  nodes: FlowNode[];
  edges: FlowEdge[];
}
```

**规则**：按 DFS 路径生成稳定节点 ID，并给出初始层级坐标。

### generatePolicyExpr(tree)

**输入**：合法 `PolicyTreeNode`。

**输出**：策略表达式字符串。

**规则**：逻辑节点按子表达式拼接，嵌套逻辑节点使用括号。

### validateTree(tree, attributes)

**输入**：业务访问树和启用属性字典。

**输出**：`ValidationError[]`。

**规则**：检查根节点、逻辑节点、叶子节点、属性启用状态和值类型。
