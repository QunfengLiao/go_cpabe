import { MarkerType, Position, type Edge, type Node } from "@xyflow/react";
import type { FlowAccessNodeData, PolicyAttribute, PolicyOperator, PolicyTreeNode, SimpleFlowEdge, SimpleFlowNode, ValidationError } from "./types";
import { attributeCode, attributeType, attributeValues } from "./types";

export type PolicyNodeType = "and" | "or" | "attribute";
export type PolicyConditionOperator = "=" | "!=" | ">" | ">=" | "<" | "<=";

export interface EditablePolicyTreeNode {
  id: string;
  type: PolicyNodeType;
  label: string;
  children?: EditablePolicyTreeNode[];
  condition?: {
    field: string;
    operator: PolicyConditionOperator;
    value: string | number;
  };
  position?: {
    x: number;
    y: number;
  };
}

const LEVEL_GAP = 170;
const SIBLING_GAP = 56;
const LOGIC_NODE_WIDTH = 210;
const ATTRIBUTE_NODE_WIDTH = 240;

export function createLogicPolicyNode(type: "and" | "or", id: string): EditablePolicyTreeNode {
  return { id, type, label: type === "and" ? "AND" : "OR", children: [] };
}

export function createAttributePolicyNode(id: string, field: string, value: string | number = ""): EditablePolicyTreeNode {
  return { id, type: "attribute", label: field || "属性条件", condition: { field, operator: "=", value } };
}

export function treeToFlow(policyTree: EditablePolicyTreeNode | null, errorByNode = new Map<string, string>(), attrByCode = new Map<string, PolicyAttribute>()): { nodes: SimpleFlowNode[]; edges: SimpleFlowEdge[] } {
  if (!policyTree) return { nodes: [], edges: [] };
  const nodes: SimpleFlowNode[] = [];
  const edges: SimpleFlowEdge[] = [];
  visitPolicyTree(policyTree, nodes, edges, errorByNode, attrByCode);
  return { nodes, edges };
}

export function policyFlowToReactFlow(nodes: SimpleFlowNode[], edges: SimpleFlowEdge[]): { nodes: Node<FlowAccessNodeData>[]; edges: Edge[] } {
  return {
    nodes: nodes.map((node) => ({
      ...node,
      width: node.data.nodeType === "LEAF" ? 220 : 170,
      height: node.data.nodeType === "LEAF" ? 78 : 82,
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top
    })) as Node<FlowAccessNodeData>[],
    edges: edges.map((edge) => ({
      ...edge,
      type: "smoothstep",
      animated: false,
      interactionWidth: 18,
      sourceHandle: "source",
      targetHandle: "target",
      style: { stroke: "#2563eb", strokeWidth: 2.5 },
      markerEnd: { type: MarkerType.ArrowClosed, color: "#2563eb", width: 18, height: 18 }
    })) as Edge[]
  };
}

function visitPolicyTree(node: EditablePolicyTreeNode, nodes: SimpleFlowNode[], edges: SimpleFlowEdge[], errorByNode: Map<string, string>, attrByCode: Map<string, PolicyAttribute>): void {
  const flowType = node.type === "and" ? "andNode" : node.type === "or" ? "orNode" : "attributeNode";
  const condition = node.condition;
  const attr = condition?.field ? attrByCode.get(condition.field) : undefined;
  nodes.push({
    id: node.id,
    type: flowType,
    position: node.position ?? { x: 0, y: 0 },
    data: node.type === "attribute"
      ? {
        nodeType: "LEAF",
        attribute: condition?.field ?? "",
        attributeName: attr?.attrName ?? attr?.attr_name ?? condition?.field ?? "",
        attributeType: attr ? attributeType(attr) : "attr",
        operator: (condition?.operator ?? "=") as PolicyOperator,
        value: condition?.value ?? "",
        label: node.label,
        error: errorByNode.get(node.id)
      }
      : {
        nodeType: node.type === "or" ? "OR" : "AND",
        label: node.label,
        error: errorByNode.get(node.id)
      }
  });
  for (const child of node.children ?? []) {
    edges.push({ id: `edge-${node.id}-${child.id}`, source: node.id, target: child.id });
    visitPolicyTree(child, nodes, edges, errorByNode, attrByCode);
  }
}

export function addChildToPolicyTree(policyTree: EditablePolicyTreeNode | null, parentId: string, childNode: EditablePolicyTreeNode): { tree: EditablePolicyTreeNode | null; added: boolean; reason?: string } {
  if (!policyTree) return { tree: null, added: false, reason: "请先创建根节点" };
  let added = false;
  let reason = "";
  const next = mapPolicyTree(policyTree, (node) => {
    if (node.id !== parentId) return node;
    if (node.type === "attribute") {
      reason = "属性节点不能添加子节点";
      return node;
    }
    added = true;
    return { ...node, children: [...(node.children ?? []), childNode] };
  });
  return { tree: next, added, reason: added ? undefined : reason || "请先选择一个 AND 或 OR 父节点" };
}

export function updatePolicyTreeNode(policyTree: EditablePolicyTreeNode | null, nodeId: string, patch: Partial<EditablePolicyTreeNode>): EditablePolicyTreeNode | null {
  if (!policyTree) return null;
  return mapPolicyTree(policyTree, (node) => {
    if (node.id !== nodeId) return node;
    const nextType = patch.type ?? node.type;
    if (nextType === "attribute") {
      return { ...node, ...patch, type: "attribute", children: undefined };
    }
    return { ...node, ...patch, type: nextType, condition: undefined, children: patch.children ?? node.children ?? [] };
  });
}

export function deletePolicyTreeNode(policyTree: EditablePolicyTreeNode | null, nodeId: string): EditablePolicyTreeNode | null {
  if (!policyTree) return null;
  if (policyTree.id === nodeId) return null;
  return mapPolicyTree(policyTree, (node) => node.type === "attribute" ? node : { ...node, children: (node.children ?? []).filter((child) => child.id !== nodeId) });
}

export function findPolicyNode(policyTree: EditablePolicyTreeNode | null, nodeId: string): EditablePolicyTreeNode | null {
  if (!policyTree) return null;
  if (policyTree.id === nodeId) return policyTree;
  for (const child of policyTree.children ?? []) {
    const found = findPolicyNode(child, nodeId);
    if (found) return found;
  }
  return null;
}

export function validateEditablePolicyTree(policyTree: EditablePolicyTreeNode | null, attributes: PolicyAttribute[]): ValidationError[] {
  if (!policyTree) return [{ path: "root", message: "访问树不能为空" }];
  const attrMap = new Map(attributes.filter((attr) => attr.status === "enabled").map((attr) => [attributeCode(attr), attr]));
  const errors: ValidationError[] = [];
  if (policyTree.type === "attribute") {
    errors.push({ nodeId: policyTree.id, path: "root", message: "根节点必须是 AND 或 OR 逻辑节点" });
  }
  errors.push(...validateEditableNode(policyTree, attrMap, "root"));
  return errors;
}

function validateEditableNode(node: EditablePolicyTreeNode, attributes: Map<string, PolicyAttribute>, path: string): ValidationError[] {
  if (node.type !== "attribute") {
    const errors: ValidationError[] = [];
    const children = node.children ?? [];
    if (children.length < 2) {
      errors.push({ nodeId: node.id, path, message: `节点「${node.label || node.type.toUpperCase()}」至少需要 2 个子条件` });
    }
    children.forEach((child, index) => errors.push(...validateEditableNode(child, attributes, `${path}.children[${index}]`)));
    return errors;
  }

  const errors: ValidationError[] = [];
  const condition = node.condition;
  if ((node.children ?? []).length > 0) {
    errors.push({ nodeId: node.id, path, message: "属性节点不能包含子节点" });
  }
  if (!condition?.field) {
    errors.push({ nodeId: node.id, path, message: "属性条件需要选择字段" });
    return errors;
  }
  const attr = attributes.get(condition.field);
  if (!attr) {
    errors.push({ nodeId: node.id, path, message: "属性未开放或不存在" });
    return errors;
  }
  if (!condition.operator) {
    errors.push({ nodeId: node.id, path, message: "属性条件需要选择操作符" });
  }
  if (condition.value === "" || condition.value === null || condition.value === undefined) {
    errors.push({ nodeId: node.id, path, message: "属性值不能为空" });
  }
  if (attributeType(attr) === "enum" && !attributeValues(attr).includes(String(condition.value))) {
    errors.push({ nodeId: node.id, path, message: "属性值不在可选值范围内" });
  }
  if (attributeType(attr) === "number" && Number.isNaN(Number(condition.value))) {
    errors.push({ nodeId: node.id, path, message: "属性值必须是数字" });
  }
  return errors;
}

export function layoutPolicyTree(policyTree: EditablePolicyTreeNode | null): EditablePolicyTreeNode | null {
  if (!policyTree) return null;
  const widths = new Map<string, number>();

  function subtreeWidth(node: EditablePolicyTreeNode): number {
    const ownWidth = node.type === "attribute" ? ATTRIBUTE_NODE_WIDTH : LOGIC_NODE_WIDTH;
    const children = node.children ?? [];
    if (children.length === 0) {
      widths.set(node.id, ownWidth);
      return ownWidth;
    }
    const childWidth = children.reduce((sum, child) => sum + subtreeWidth(child), 0) + SIBLING_GAP * (children.length - 1);
    const width = Math.max(ownWidth, childWidth);
    widths.set(node.id, width);
    return width;
  }

  function place(node: EditablePolicyTreeNode, depth: number, left: number): EditablePolicyTreeNode {
    const width = widths.get(node.id) ?? (node.type === "attribute" ? ATTRIBUTE_NODE_WIDTH : LOGIC_NODE_WIDTH);
    const ownWidth = node.type === "attribute" ? ATTRIBUTE_NODE_WIDTH : LOGIC_NODE_WIDTH;
    let childLeft = left;
    const children = (node.children ?? []).map((child) => {
      const childWidth = widths.get(child.id) ?? ATTRIBUTE_NODE_WIDTH;
      const nextChild = place(child, depth + 1, childLeft);
      childLeft += childWidth + SIBLING_GAP;
      return nextChild;
    });
    return {
      ...node,
      position: { x: left + width / 2 - ownWidth / 2 + 48, y: depth * LEVEL_GAP + 36 },
      children: node.type === "attribute" ? undefined : children
    };
  }

  subtreeWidth(policyTree);
  return place(policyTree, 0, 36);
}

export function editableToBackendTree(policyTree: EditablePolicyTreeNode | null): PolicyTreeNode | null {
  if (!policyTree) return null;
  if (policyTree.type === "attribute") {
    return {
      type: "LEAF",
      attribute: policyTree.condition?.field ?? "",
      operator: policyTree.condition?.operator === "!=" ? "!=" : "=",
      value: policyTree.condition?.value ?? ""
    };
  }
  return { type: policyTree.type === "or" ? "OR" : "AND", children: (policyTree.children ?? []).map((child) => editableToBackendTree(child)).filter(Boolean) as PolicyTreeNode[] };
}

export function backendToEditableTree(tree: PolicyTreeNode | null, prefix = "root"): EditablePolicyTreeNode | null {
  if (!tree) return null;
  if (tree.type === "LEAF") {
    return createAttributePolicyNode(prefix, tree.attribute, tree.value);
  }
  return {
    id: prefix,
    type: tree.type === "OR" ? "or" : "and",
    label: tree.type,
    children: tree.children.map((child, index) => backendToEditableTree(child, `${prefix}-${index}`)).filter(Boolean) as EditablePolicyTreeNode[]
  };
}

export function generateEditablePolicyExpression(policyTree: EditablePolicyTreeNode | null): string {
  const backendTree = editableToBackendTree(policyTree);
  if (!backendTree) return "";
  return generateBackendExpression(backendTree);
}

function generateBackendExpression(tree: PolicyTreeNode): string {
  if (tree.type === "LEAF") {
    const value = String(tree.value ?? "");
    return tree.operator === "!=" ? `${tree.attribute}!=${quoteIfNeeded(value)}` : `${tree.attribute}:${quoteIfNeeded(value)}`;
  }
  return tree.children.map((child) => {
    const expr = generateBackendExpression(child);
    return child.type === "LEAF" ? expr : `(${expr})`;
  }).join(` ${tree.type} `);
}

function quoteIfNeeded(value: string): string {
  if (!value) return "\"\"";
  return /\s|[()]/.test(value) ? JSON.stringify(value) : value;
}

function mapPolicyTree(node: EditablePolicyTreeNode, mapper: (node: EditablePolicyTreeNode) => EditablePolicyTreeNode): EditablePolicyTreeNode {
  const mappedChildren = node.children?.map((child) => mapPolicyTree(child, mapper));
  return mapper({ ...node, children: node.type === "attribute" ? undefined : mappedChildren ?? [] });
}
