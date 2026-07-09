import type { PolicyTreeNode, SimpleFlowEdge, SimpleFlowNode, ValidationError } from "./types";

const X_GAP = 220;
const Y_GAP = 130;

export function treeToFlow(tree: PolicyTreeNode | null): { nodes: SimpleFlowNode[]; edges: SimpleFlowEdge[] } {
  if (!tree) return { nodes: [], edges: [] };
  const nodes: SimpleFlowNode[] = [];
  const edges: SimpleFlowEdge[] = [];
  visitTree(tree, "root", 0, 0, nodes, edges);
  return { nodes, edges };
}

export const buildFlowFromPolicyTree = treeToFlow;

function visitTree(node: PolicyTreeNode, id: string, depth: number, order: number, nodes: SimpleFlowNode[], edges: SimpleFlowEdge[]): void {
  const type = node.type === "AND" ? "andNode" : node.type === "OR" ? "orNode" : "attributeNode";
  nodes.push({
    id,
    type,
    position: { x: depth * X_GAP, y: order * Y_GAP },
    data: node.type === "LEAF"
      ? { nodeType: "LEAF", attribute: node.attribute, operator: node.operator, value: node.value }
      : { nodeType: node.type, label: node.type }
  });
  if (node.type !== "LEAF") {
    node.children.forEach((child, index) => {
      const childId = `${id}-${index}`;
      edges.push({ id: `edge-${id}-${childId}`, source: id, target: childId });
      visitTree(child, childId, depth + 1, order + index, nodes, edges);
    });
  }
}

export function flowToTree(nodes: SimpleFlowNode[], edges: SimpleFlowEdge[]): { tree: PolicyTreeNode | null; errors: ValidationError[] } {
  if (nodes.length === 0) return { tree: null, errors: [{ path: "root", message: "访问树不能为空" }] };
  const incoming = new Map(nodes.map((node) => [node.id, 0]));
  const children = new Map(nodes.map((node) => [node.id, [] as string[]]));
  for (const edge of edges) {
    if (!incoming.has(edge.target) || !children.has(edge.source)) continue;
    incoming.set(edge.target, (incoming.get(edge.target) ?? 0) + 1);
    children.get(edge.source)!.push(edge.target);
  }
  const roots = [...incoming.entries()].filter(([, count]) => count === 0).map(([id]) => id);
  if (roots.length !== 1) return { tree: null, errors: [{ path: "root", message: "访问树必须且只能有一个根节点" }] };
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const visiting = new Set<string>();
  const visited = new Set<string>();
  const errors: ValidationError[] = [];

  function build(id: string, path: string): PolicyTreeNode | null {
    if (visiting.has(id)) {
      errors.push({ nodeId: id, path, message: "访问树不能出现循环引用" });
      return null;
    }
    const node = byId.get(id);
    if (!node) return null;
    visiting.add(id);
    visited.add(id);
    const childIds = (children.get(id) ?? []).sort((a, b) => (byId.get(a)?.position.x ?? 0) - (byId.get(b)?.position.x ?? 0));
    let result: PolicyTreeNode;
    if (node.data.nodeType === "LEAF") {
      result = { type: "LEAF", attribute: String(node.data.attribute ?? ""), operator: node.data.operator ?? "=", value: node.data.value ?? "" };
    } else {
      result = { type: node.data.nodeType === "OR" ? "OR" : "AND", children: childIds.map((childId, index) => build(childId, `${path}.children[${index}]`)).filter(Boolean) as PolicyTreeNode[] };
    }
    visiting.delete(id);
    return result;
  }

  const tree = build(roots[0], "root");
  if (visited.size !== nodes.length) errors.push({ path: "root", message: "访问树不能存在孤立节点" });
  return { tree, errors };
}

export const buildPolicyTreeFromFlow = flowToTree;
