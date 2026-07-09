import { useEffect, useMemo, useRef } from "react";
import {
  Background,
  BackgroundVariant,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type ReactFlowInstance
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { AndNode } from "./nodes/AndNode";
import { AttributeNode } from "./nodes/AttributeNode";
import { OrNode } from "./nodes/OrNode";
import {
  attributeCode,
  attributeName,
  attributeType,
  type FlowAccessNodeData,
  type PolicyAttribute,
  type SimpleFlowEdge,
  type SimpleFlowNode,
  type ValidationError
} from "./tree/types";
import { normalizeNodePositions } from "./tree/layout";

const previewNodeTypes = { andNode: AndNode, orNode: OrNode, attributeNode: AttributeNode };

export function AccessTreePreviewCanvas({
  nodes,
  edges,
  errors,
  attributes,
  onOpenEditor
}: {
  nodes: SimpleFlowNode[];
  edges: SimpleFlowEdge[];
  errors: ValidationError[];
  attributes: PolicyAttribute[];
  onOpenEditor: () => void;
}) {
  const flowRef = useRef<ReactFlowInstance<Node<FlowAccessNodeData>, Edge> | null>(null);
  const normalizedNodes = useMemo(() => normalizeNodePositions(nodes), [nodes]);
  const errorByNode = useMemo(() => new Map(errors.filter((error) => error.nodeId).map((error) => [error.nodeId!, error.message])), [errors]);
  const attrMap = useMemo(() => new Map(attributes.map((attribute) => [attributeCode(attribute), attribute])), [attributes]);
  const validEdges = useMemo(() => sanitizeEdges(edges, normalizedNodes, "AccessTreePreviewCanvas"), [edges, normalizedNodes]);

  const flowNodes = useMemo(() => normalizedNodes.map((node) => {
    const attr = node.data.attribute ? attrMap.get(node.data.attribute) : undefined;
    return {
      ...node,
      draggable: false,
      selectable: false,
      width: node.data.nodeType === "LEAF" ? 220 : 170,
      height: node.data.nodeType === "LEAF" ? 78 : 82,
      sourcePosition: Position.Bottom,
      targetPosition: Position.Top,
      data: {
        ...node.data,
        attributeName: attr ? attributeName(attr) : undefined,
        attributeType: attr ? attributeType(attr) : undefined,
        error: errorByNode.get(node.id)
      }
    };
  }) as Node<FlowAccessNodeData>[], [attrMap, errorByNode, normalizedNodes]);

  const flowEdges = useMemo(() => validEdges.map((edge) => ({
    ...edge,
    type: "smoothstep",
    interactionWidth: 18,
    style: { stroke: "#5f7f9f", strokeWidth: 2.5 },
    markerEnd: { type: MarkerType.ArrowClosed, color: "#5f7f9f", width: 18, height: 18 }
  })) as Edge[], [validEdges]);

  useEffect(() => {
    console.debug("[AccessTreePreviewCanvas] flow data", { nodes: normalizedNodes, edges, validEdges });
  }, [edges, normalizedNodes, validEdges]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      flowRef.current?.fitView({ padding: 0.2, duration: 250, includeHiddenNodes: true });
    }, 80);
    return () => window.clearTimeout(timer);
  }, [normalizedNodes.length, flowEdges.length]);

  return (
    <button className="access-tree-preview-canvas" type="button" onClick={onOpenEditor} aria-label="打开可视化编辑器">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={previewNodeTypes}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag={false}
        zoomOnScroll={false}
        zoomOnPinch={false}
        zoomOnDoubleClick={false}
        preventScrolling={false}
        minZoom={0.25}
        maxZoom={1.2}
        onInit={(instance) => {
          flowRef.current = instance;
        }}
        defaultEdgeOptions={{
          type: "smoothstep",
          style: { stroke: "#5f7f9f", strokeWidth: 2.5 },
          markerEnd: { type: MarkerType.ArrowClosed, color: "#5f7f9f" }
        }}
      >
        <Background variant={BackgroundVariant.Lines} gap={36} size={1} color="#d6e1ee" />
        {nodes.length === 0 && (
          <div className="preview-canvas-empty">
            <strong>尚未生成访问树</strong>
            <span>进入可视化编辑器后添加逻辑节点和属性条件</span>
          </div>
        )}
      </ReactFlow>
    </button>
  );
}

function sanitizeEdges(edges: SimpleFlowEdge[], nodes: SimpleFlowNode[], scope: string): SimpleFlowEdge[] {
  const nodeIds = new Set(nodes.map((node) => node.id));
  const validEdges = edges.filter((edge) => nodeIds.has(edge.source) && nodeIds.has(edge.target));
  if (validEdges.length !== edges.length) {
    console.warn(`[${scope}] dropped invalid edges`, {
      edges,
      nodeIds: [...nodeIds],
      invalidEdges: edges.filter((edge) => !nodeIds.has(edge.source) || !nodeIds.has(edge.target))
    });
  }
  return validEdges;
}
