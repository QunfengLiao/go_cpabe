import { useEffect, useMemo, useRef, useState } from "react";
import { AccessTreeCanvas } from "./AccessTreeCanvas";
import { AccessTreeSidebar } from "./AccessTreeSidebar";
import { AccessTreeToolbar } from "./AccessTreeToolbar";
import { PolicyPreviewTabs } from "./PolicyPreviewTabs";
import { PolicySettingsModal } from "./PolicySettingsModal";
import { createAccessPolicy, getAccessPolicy, updateAccessPolicy } from "../../api/policy";
import { ApiError } from "../../api/request";
import { clearDraft, draftKey, loadDraft, saveDraft } from "./tree/draft";
import { mockAttributes, mockTemplates, mockTree } from "./tree/mockData";
import {
  addChildToPolicyTree,
  backendToEditableTree,
  createAttributePolicyNode,
  createLogicPolicyNode,
  deletePolicyTreeNode,
  editableToBackendTree,
  findPolicyNode,
  generateEditablePolicyExpression,
  layoutPolicyTree,
  treeToFlow,
  updatePolicyTreeNode,
  validateEditablePolicyTree,
  type EditablePolicyTreeNode
} from "./tree/policyModel";
import { attributeCode, attributeName, attributeValues, policyTree, templateTree, type AccessPolicy, type PolicyAttribute, type PolicyTemplate, type SimpleFlowEdge, type SimpleFlowNode, type ValidationError } from "./tree/types";

export function AccessTreeEditor({
  mode,
  policyId,
  tenantId,
  attributes = mockAttributes,
  templates = mockTemplates,
  variant = "embedded",
  onBack,
  onSaved
}: {
  mode: "create" | "edit";
  policyId?: string;
  tenantId: string;
  attributes?: PolicyAttribute[];
  templates?: PolicyTemplate[];
  variant?: "embedded" | "workbench";
  onBack?: () => void;
  onSaved?: (policy: AccessPolicy) => void;
}) {
  const draftStorageKey = draftKey("current", tenantId || "tenant", policyId ?? "new");
  const idSeedRef = useRef(0);
  const [name, setName] = useState("研发部数据拥有者访问策略");
  const [description, setDescription] = useState("研发部数据拥有者或租户管理员可访问");
  const [policyTreeState, setPolicyTreeState] = useState<EditablePolicyTreeNode | null>(() => mode === "create" ? layoutPolicyTree(backendToEditableTree(mockTree)) : null);
  const [selectedNodeId, setSelectedNodeId] = useState(mode === "create" ? "root" : "");
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [fitViewVersion, setFitViewVersion] = useState(1);
  const [previewCollapsed, setPreviewCollapsed] = useState(variant === "workbench");
  const [settingsOpen, setSettingsOpen] = useState(false);

  useEffect(() => {
    if (mode !== "edit" || !policyId || !tenantId) return;
    getAccessPolicy(tenantId, policyId).then((policy) => {
      setName(policy.name);
      setDescription(policy.description ?? "");
      const tree = policyTree(policy);
      const editableTree = layoutPolicyTree(backendToEditableTree(tree));
      setPolicyTreeState(editableTree);
      setSelectedNodeId(editableTree?.id ?? "");
      setFitViewVersion((value) => value + 1);
      setDirty(false);
      debugTreeState("加载策略后", editableTree);
    }).catch(() => setMessage("策略详情加载失败"));
  }, [mode, policyId, tenantId]);

  useEffect(() => {
    const draft = loadDraft(draftStorageKey);
    if (!draft || !window.confirm("发现未保存草稿，是否恢复？")) return;
    setName(draft.name);
    setDescription(draft.description);
    const draftTree = draft.policyTree ? layoutPolicyTree(draft.policyTree) : layoutPolicyTree(rebuildDraftTree(draft.nodes, draft.edges));
    setPolicyTreeState(draftTree);
    setSelectedNodeId(draft.selectedNodeId);
    setDirty(true);
    debugTreeState("恢复草稿后", draftTree);
  }, [draftStorageKey]);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (!event.ctrlKey) return;
      if (event.key.toLowerCase() === "s") {
        event.preventDefault();
        void save();
      }
      if (event.key === "0") {
        event.preventDefault();
        setFitViewVersion((value) => value + 1);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  });

  const attrMap = useMemo(() => new Map(attributes.map((attribute) => [attributeCode(attribute), attribute])), [attributes]);
  const validationErrors = useMemo(() => normalizeValidationErrors(validateEditablePolicyTree(policyTreeState, attributes), policyTreeState, attributes), [attributes, policyTreeState]);
  const errorByNode = useMemo(() => new Map(validationErrors.filter((error) => error.nodeId).map((error) => [error.nodeId!, error.message])), [validationErrors]);
  const flow = useMemo(() => treeToFlow(policyTreeState, errorByNode, attrMap), [attrMap, errorByNode, policyTreeState]);
  const backendTree = useMemo(() => editableToBackendTree(policyTreeState), [policyTreeState]);
  const expression = useMemo(() => generateEditablePolicyExpression(policyTreeState), [policyTreeState]);
  const selectedNode = useMemo(() => findPolicyNode(policyTreeState, selectedNodeId), [policyTreeState, selectedNodeId]);

  useEffect(() => {
    debugTreeState("policyTree 更新后", policyTreeState);
    debugFlowState("policyTree 派生 flow", flow.nodes, flow.edges);
  }, [flow.edges, flow.nodes, policyTreeState]);

  function addNode(type: "AND" | "OR" | "LEAF", attribute?: string) {
    if (!policyTreeState) {
      if (type === "LEAF") {
        setMessage("请先创建 AND 或 OR 根节点");
        return;
      }
      const root = layoutPolicyTree(createLogicPolicyNode(type === "OR" ? "or" : "and", "root"));
      setPolicyTreeState(root);
      setSelectedNodeId("root");
      setDirty(true);
      setMessage("已创建根节点");
      debugTreeState("创建根节点后", root);
      setFitViewVersion((value) => value + 1);
      return;
    }

    if (!selectedNodeId) {
      setMessage("请先选择一个 AND 或 OR 父节点");
      return;
    }
    const parent = findPolicyNode(policyTreeState, selectedNodeId);
    if (!parent) {
      setMessage("请先选择一个 AND 或 OR 父节点");
      return;
    }
    if (parent.type === "attribute") {
      setMessage("属性节点不能添加子节点");
      return;
    }

    const child = createChildNode(type, attribute);
    const result = addChildToPolicyTree(policyTreeState, parent.id, child);
    if (!result.added) {
      setMessage(result.reason ?? "添加失败");
      return;
    }
    const nextTree = layoutPolicyTree(result.tree);
    setPolicyTreeState(nextTree);
    setSelectedNodeId(parent.id);
    setDirty(true);
    setMessage(`已添加到 ${parent.label || parent.type.toUpperCase()} 节点下`);
    debugTreeState("添加节点后", nextTree);
    setFitViewVersion((value) => value + 1);
  }

  function createChildNode(type: "AND" | "OR" | "LEAF", attribute?: string): EditablePolicyTreeNode {
    const id = nextNodeId(type, idSeedRef);
    if (type !== "LEAF") return createLogicPolicyNode(type === "OR" ? "or" : "and", id);
    const attr = attribute ? attributes.find((item) => attributeCode(item) === attribute) : undefined;
    return createAttributePolicyNode(id, attribute ?? "", attr ? attributeValues(attr)[0] ?? "" : "");
  }

  function updateNode(nodeId: string, patch: Partial<EditablePolicyTreeNode>) {
    const nextTree = updatePolicyTreeNode(policyTreeState, nodeId, patch);
    setPolicyTreeState(nextTree);
    setDirty(true);
    debugTreeState("更新节点后", nextTree);
  }

  function updateNodePosition(nodeId: string, position: { x: number; y: number }) {
    const nextTree = updatePolicyTreeNode(policyTreeState, nodeId, { position });
    setPolicyTreeState(nextTree);
    setDirty(true);
  }

  function deleteNode(nodeId: string) {
    const node = findPolicyNode(policyTreeState, nodeId);
    if (!node) return;
    if ((node.children ?? []).length > 0 && !window.confirm("该节点包含子节点，是否同时删除子节点？")) return;
    const nextTree = layoutPolicyTree(deletePolicyTreeNode(policyTreeState, nodeId));
    setPolicyTreeState(nextTree);
    setSelectedNodeId(nextTree?.id ?? "");
    setDirty(true);
    setMessage("节点已删除");
    debugTreeState("删除节点后", nextTree);
  }

  function useExampleTemplate() {
    const nextTree = layoutPolicyTree(backendToEditableTree(mockTree));
    setPolicyTreeState(nextTree);
    setSelectedNodeId(nextTree?.id ?? "");
    setDirty(true);
    setFitViewVersion((value) => value + 1);
    setMessage("已载入示例访问树");
    debugTreeState("使用示例模板后", nextTree);
  }

  function applyTemplate(template: PolicyTemplate) {
    const tree = templateTree(template);
    if (!tree) return;
    const nextTree = layoutPolicyTree(backendToEditableTree(tree));
    setPolicyTreeState(nextTree);
    setSelectedNodeId(nextTree?.id ?? "");
    setDirty(true);
    setFitViewVersion((value) => value + 1);
    setMessage(`已载入模板：${template.name}`);
    debugTreeState("使用模板后", nextTree);
  }

  function autoLayout() {
    debugTreeState("自动布局前", policyTreeState);
    const nextTree = layoutPolicyTree(policyTreeState);
    setPolicyTreeState(nextTree);
    setFitViewVersion((value) => value + 1);
    setMessage("访问树布局已整理");
    debugTreeState("自动布局后", nextTree);
  }

  function validateCurrentTree() {
    debugTreeState("点击校验前", policyTreeState);
    setPreviewCollapsed(false);
    setMessage(validationErrors.length > 0 ? `当前访问树存在 ${validationErrors.length} 个问题` : "当前访问树校验通过");
  }

  function saveDraftToLocal() {
    const latestFlow = treeToFlow(policyTreeState, errorByNode, attrMap);
    saveDraft(draftStorageKey, { name, description, nodes: latestFlow.nodes, edges: latestFlow.edges, selectedNodeId, policyTree: policyTreeState, savedAt: Date.now() });
    setMessage("草稿已保存到本机");
  }

  async function save() {
    if (!backendTree || validationErrors.length > 0) {
      setMessage("请先修正访问树错误");
      return;
    }
    setSaving(true);
    setMessage("");
    try {
      const payload = { name, description, policyExpr: expression, policyTreeJson: backendTree, status: "enabled" as const };
      console.debug("[AccessTreeEditor] save policy payload", payload);
      const policy = mode === "edit" && policyId ? await updateAccessPolicy(tenantId, policyId, payload) : await createAccessPolicy(tenantId, payload);
      setDirty(false);
      clearDraft(draftStorageKey);
      setMessage("保存成功");
      onSaved?.(policy);
    } catch (err) {
      if (err instanceof ApiError) {
        setMessage(`保存失败：${err.message}${err.code ? `（${err.code}）` : ""}`);
      } else {
        setMessage("保存失败，请检查权限和访问树");
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className={`access-tree-editor access-tree-editor-${variant}${previewCollapsed ? " preview-collapsed" : ""}`}>
      <div className="access-tree-header">
        {variant === "workbench" && <button className="editor-back-button" type="button" onClick={onBack}>返回</button>}
        <div className="access-tree-title-block">
          <span>访问策略构建</span>
          <h2>{name}</h2>
          <p>{dirty ? "未保存更改" : "策略已同步"}</p>
        </div>
        <button className="toolbar-secondary policy-settings-trigger" type="button" onClick={() => setSettingsOpen(true)}>策略设置</button>
        <AccessTreeToolbar dirty={dirty} saving={saving} hasErrors={validationErrors.length > 0} onSave={() => void save()} onValidate={validateCurrentTree} onAutoLayout={autoLayout} onFitView={() => setFitViewVersion((value) => value + 1)} />
      </div>
      {message && <div className="access-tree-toast">{message}</div>}
      <div className="access-tree-workbench">
        <AccessTreeSidebar attributes={attributes} templates={templates} dirty={dirty} onAddLogic={(type) => addNode(type)} onCreateAttributeNode={(code) => addNode("LEAF", code)} onApplyTemplate={applyTemplate} />
        <AccessTreeCanvas
          nodes={flow.nodes}
          edges={flow.edges}
          selectedNode={selectedNode}
          selectedNodeId={selectedNodeId}
          attributes={attributes}
          fitViewVersion={fitViewVersion}
          onUseExampleTemplate={useExampleTemplate}
          onSelectNode={setSelectedNodeId}
          onChangeNode={updateNode}
          onDeleteNode={deleteNode}
          onUpdateNodePosition={updateNodePosition}
        />
      </div>
      <div className="access-tree-footer">
        {variant === "workbench" && (
          <button className="draft-action" type="button" onClick={() => setPreviewCollapsed((collapsed) => !collapsed)}>
            {previewCollapsed ? "展开预览" : "折叠预览"}
          </button>
        )}
        <button className="draft-action" type="button" onClick={saveDraftToLocal}>保存本机草稿</button>
        {!previewCollapsed && <PolicyPreviewTabs expression={expression} tree={backendTree} errors={validationErrors} />}
      </div>
      {settingsOpen && (
        <PolicySettingsModal
          name={name}
          description={description}
          onNameChange={(value) => { setName(value); setDirty(true); }}
          onDescriptionChange={(value) => { setDescription(value); setDirty(true); }}
          onClose={() => setSettingsOpen(false)}
        />
      )}
    </div>
  );
}

function nextNodeId(type: "AND" | "OR" | "LEAF", idSeedRef: React.MutableRefObject<number>): string {
  idSeedRef.current += 1;
  return `${type.toLowerCase()}-${Date.now()}-${idSeedRef.current}`;
}

function debugTreeState(action: string, tree: EditablePolicyTreeNode | null): void {
  console.log(`[AccessTreeEditor] ${action} policyTree:`, tree);
}

function debugFlowState(action: string, nodes: SimpleFlowNode[], edges: SimpleFlowEdge[]): void {
  console.log(`[AccessTreeEditor] ${action} nodes:`, nodes.length, nodes);
  console.log(`[AccessTreeEditor] ${action} edges:`, edges.length, edges);
}

function normalizeValidationErrors(errors: ValidationError[], tree: EditablePolicyTreeNode | null, attributes: PolicyAttribute[]): ValidationError[] {
  const attrMap = new Map(attributes.map((attribute) => [attributeCode(attribute), attributeName(attribute)]));
  return errors.map((error) => {
    const node = error.nodeId ? findPolicyNode(tree, error.nodeId) : null;
    return {
      ...error,
      message: friendlyValidationMessage(error.message, node, attrMap)
    };
  });
}

function friendlyValidationMessage(message: string, node: EditablePolicyTreeNode | null, attrMap: Map<string, string>): string {
  if (message.includes("至少需要")) {
    const label = node?.type === "or" ? "OR" : "AND";
    return `节点「${label}」至少需要 2 个子条件`;
  }
  if (message.includes("属性") || message.includes("字段")) {
    const field = node?.condition?.field ? attrMap.get(node.condition.field) ?? node.condition.field : "当前属性";
    return `属性「${field}」配置不完整或不可用`;
  }
  if (message.includes("根节点")) return "访问树根节点必须是 AND 或 OR";
  if (message.includes("不能为空")) return "访问树需要至少一个根节点";
  return message;
}

function rebuildDraftTree(nodes: SimpleFlowNode[], edges: SimpleFlowEdge[]): EditablePolicyTreeNode | null {
  if (nodes.length === 0) return null;
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const incoming = new Map(nodes.map((node) => [node.id, 0]));
  const children = new Map(nodes.map((node) => [node.id, [] as string[]]));
  for (const edge of edges) {
    if (!incoming.has(edge.target) || !children.has(edge.source)) continue;
    incoming.set(edge.target, (incoming.get(edge.target) ?? 0) + 1);
    children.get(edge.source)!.push(edge.target);
  }
  const rootId = [...incoming.entries()].find(([, count]) => count === 0)?.[0] ?? nodes[0].id;
  function build(id: string): EditablePolicyTreeNode | null {
    const node = byId.get(id);
    if (!node) return null;
    if (node.data.nodeType === "LEAF") {
      return {
        id,
        type: "attribute",
        label: String(node.data.label ?? node.data.attribute ?? "属性条件"),
        condition: { field: String(node.data.attribute ?? ""), operator: node.data.operator === "!=" ? "!=" : "=", value: node.data.value ?? "" },
        position: node.position
      };
    }
    return {
      id,
      type: node.data.nodeType === "OR" ? "or" : "and",
      label: String(node.data.label ?? node.data.nodeType),
      position: node.position,
      children: (children.get(id) ?? []).map(build).filter(Boolean) as EditablePolicyTreeNode[]
    };
  }
  return build(rootId);
}
