import { useEffect, useMemo, useRef, useState } from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  MiniMap,
  NodeToolbar,
  Position,
  ReactFlow,
  applyNodeChanges,
  type Edge,
  type Node,
  type ReactFlowInstance
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { AndNode } from "./nodes/AndNode";
import { AttributeNode } from "./nodes/AttributeNode";
import { OrNode } from "./nodes/OrNode";
import { NodeConfigPopover } from "./NodeConfigPopover";
import type { EditablePolicyTreeNode } from "./tree/policyModel";
import type { FlowAccessNodeData, PolicyAttribute, SimpleFlowEdge, SimpleFlowNode } from "./tree/types";

const nodeTypes = { andNode: AndNode, orNode: OrNode, attributeNode: AttributeNode };

export function AccessTreeCanvas({
  nodes,
  edges,
  selectedNode,
  selectedNodeId,
  attributes,
  fitViewVersion,
  onUseExampleTemplate,
  onSelectNode,
  onChangeNode,
  onDeleteNode,
  onUpdateNodePosition
}: {
  nodes: SimpleFlowNode[];
  edges: SimpleFlowEdge[];
  selectedNode: EditablePolicyTreeNode | null;
  selectedNodeId: string;
  attributes: PolicyAttribute[];
  fitViewVersion: number;
  onUseExampleTemplate: () => void;
  onSelectNode: (nodeId: string) => void;
  onChangeNode: (nodeId: string, patch: Partial<EditablePolicyTreeNode>) => void;
  onDeleteNode: (nodeId: string) => void;
  onUpdateNodePosition: (nodeId: string, position: { x: number; y: number }) => void;
}) {
  const flowRef = useRef<ReactFlowInstance<Node<FlowAccessNodeData>, Edge> | null>(null);
  const fitViewTimerRef = useRef<number | null>(null);
  const draggingRef = useRef(false);
  const [localNodes, setLocalNodes] = useState<SimpleFlowNode[]>(nodes);
  const [isDragging, setIsDragging] = useState(false);
  const selectedChildCount = selectedNode?.children?.length ?? 0;

  useEffect(() => {
    if (draggingRef.current) return;
    setLocalNodes(nodes);
  }, [nodes]);

  const visibleNodes = isDragging ? localNodes : nodes;
  const flowNodes = useMemo(() => visibleNodes.map((node) => ({ ...node, selected: node.id === selectedNodeId })) as Node<FlowAccessNodeData>[], [selectedNodeId, visibleNodes]);
  const flowEdges = useMemo(() => edges.map((edge) => ({
    ...edge,
    type: "smoothstep",
    animated: false,
    interactionWidth: 18,
    sourceHandle: "source",
    targetHandle: "target",
    style: { stroke: "#2563eb", strokeWidth: 2.5 },
    markerEnd: { type: MarkerType.ArrowClosed, color: "#2563eb", width: 18, height: 18 }
  })) as Edge[], [edges]);

  useEffect(() => {
    console.debug("[AccessTreeCanvas] render flow", { nodes, edges });
  }, [edges, nodes]);

  useEffect(() => {
    if (nodes.length === 0) return;
    if (fitViewTimerRef.current) window.clearTimeout(fitViewTimerRef.current);
    fitViewTimerRef.current = window.setTimeout(() => {
      flowRef.current?.fitView({ padding: 0.2, duration: 300, includeHiddenNodes: true });
      fitViewTimerRef.current = null;
    }, 80);
    return () => {
      if (fitViewTimerRef.current) window.clearTimeout(fitViewTimerRef.current);
      fitViewTimerRef.current = null;
    };
  }, [fitViewVersion, nodes.length]);

  return (
    <div className="access-tree-canvas">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        fitView={false}
        minZoom={0.25}
        maxZoom={1.6}
        onInit={(instance) => {
          flowRef.current = instance;
        }}
        defaultEdgeOptions={{
          type: "smoothstep",
          animated: false,
          style: { stroke: "#2563eb", strokeWidth: 2.5 },
          markerEnd: { type: MarkerType.ArrowClosed, color: "#2563eb" }
        }}
        onNodeClick={(_, node) => onSelectNode(node.id)}
        onPaneClick={() => onSelectNode("")}
        onNodesChange={(changes) => {
          const changed = applyNodeChanges(changes, flowNodes) as Node<FlowAccessNodeData>[];
          setLocalNodes(changed.map((node) => ({
            id: node.id,
            type: node.type as SimpleFlowNode["type"],
            position: node.position,
            data: node.data
          })));
        }}
        onNodeDragStart={() => {
          draggingRef.current = true;
          setIsDragging(true);
        }}
        onNodeDragStop={(_, node) => {
          draggingRef.current = false;
          setIsDragging(false);
          onUpdateNodePosition(node.id, node.position);
        }}
      >
        {nodes.length > 0 && <div className="canvas-node-count">当前 {nodes.length} 个节点</div>}
        <Background variant={BackgroundVariant.Lines} gap={34} size={1} color="#d6e1ee" />
        {!isDragging && <MiniMap pannable zoomable position="bottom-right" className="access-tree-minimap" />}
        <Controls position="bottom-left" />
        {selectedNode && !isDragging && (
          <NodeToolbar nodeId={selectedNode.id} isVisible position={Position.Right} offset={14}>
            <NodeConfigPopover node={selectedNode} attributes={attributes} childCount={selectedChildCount} onChange={onChangeNode} onDelete={onDeleteNode} />
          </NodeToolbar>
        )}
        {nodes.length === 0 && (
          <div className="canvas-empty-state">
            <span>ACCESS TREE</span>
            <strong>从左侧添加 AND / OR / 属性节点开始构建策略</strong>
            <button type="button" onClick={onUseExampleTemplate}>使用示例模板</button>
          </div>
        )}
      </ReactFlow>
    </div>
  );
}
