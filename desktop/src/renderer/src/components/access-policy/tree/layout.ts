import type { SimpleFlowEdge, SimpleFlowNode } from "./types";

const LEVEL_GAP = 170;
const SIBLING_GAP = 48;
const LOGIC_NODE_WIDTH = 210;
const ATTRIBUTE_NODE_WIDTH = 240;
const DEFAULT_NODE_HEIGHT = 104;

export function applyAutoLayout(nodes: SimpleFlowNode[], edges: SimpleFlowEdge[]): SimpleFlowNode[] {
  if (nodes.length === 0) return nodes;
  const children = new Map(nodes.map((node) => [node.id, [] as string[]]));
  const incoming = new Map(nodes.map((node) => [node.id, 0]));
  for (const edge of edges) {
    if (!children.has(edge.source) || !incoming.has(edge.target)) continue;
    children.get(edge.source)!.push(edge.target);
    incoming.set(edge.target, (incoming.get(edge.target) ?? 0) + 1);
  }
  const rootId = [...incoming.entries()].find(([, count]) => count === 0)?.[0] ?? nodes[0].id;
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const positioned = new Map<string, { x: number; y: number }>();

  function subtreeWidth(id: string): number {
    const node = byId.get(id);
    if (!node) return LOGIC_NODE_WIDTH;
    const childIds = children.get(id) ?? [];
    const ownWidth = nodeWidth(node);
    if (childIds.length === 0) return ownWidth;
    const childrenWidth = childIds.reduce((sum, childId) => sum + subtreeWidth(childId), 0) + SIBLING_GAP * (childIds.length - 1);
    return Math.max(ownWidth, childrenWidth);
  }

  function place(id: string, depth: number, left: number): void {
    const node = byId.get(id);
    if (!node) return;
    const width = subtreeWidth(id);
    positioned.set(id, { x: left + width / 2 - nodeWidth(node) / 2, y: depth * LEVEL_GAP + 36 });
    let childLeft = left;
    for (const childId of children.get(id) ?? []) {
      const childWidth = subtreeWidth(childId);
      place(childId, depth + 1, childLeft);
      childLeft += childWidth + SIBLING_GAP;
    }
  }

  place(rootId, 0, 36);
  let orphanIndex = 0;
  for (const node of nodes) {
    if (!positioned.has(node.id)) {
      positioned.set(node.id, { x: 36 + orphanIndex * (ATTRIBUTE_NODE_WIDTH + SIBLING_GAP), y: LEVEL_GAP * 3 + 36 });
      orphanIndex += 1;
    }
  }

  const minX = Math.min(...[...positioned.values()].map((position) => position.x));
  const minY = Math.min(...[...positioned.values()].map((position) => position.y));
  return normalizeNodePositions(nodes.map((node) => {
    const position = positioned.get(node.id) ?? node.position;
    return byId.has(node.id) ? { ...node, position: { x: position.x - minX + 48, y: position.y - minY + 36 } } : node;
  }));
}

export function normalizeNodePositions(nodes: SimpleFlowNode[]): SimpleFlowNode[] {
  return nodes.map((node, index) => {
    const x = safeCoordinate(node.position?.x, 36 + (index % 4) * ATTRIBUTE_NODE_WIDTH);
    const y = safeCoordinate(node.position?.y, 36 + Math.floor(index / 4) * DEFAULT_NODE_HEIGHT);
    return { ...node, position: { x, y } };
  });
}

function nodeWidth(node: SimpleFlowNode): number {
  return node.data.nodeType === "LEAF" ? ATTRIBUTE_NODE_WIDTH : LOGIC_NODE_WIDTH;
}

function safeCoordinate(value: unknown, fallback: number): number {
  if (typeof value !== "number" || Number.isNaN(value) || !Number.isFinite(value)) return fallback;
  if (value < -2000 || value > 20000) return fallback;
  return value;
}
