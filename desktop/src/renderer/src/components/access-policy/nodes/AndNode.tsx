import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { FlowAccessNodeData } from "../tree/types";

function AndNodeBase({ data, selected }: NodeProps) {
  const node = data as FlowAccessNodeData;
  return (
    <div className={`access-tree-node logic-node and-node${selected ? " selected" : ""}${node.error ? " invalid" : ""}`}>
      <Handle id="target" type="target" position={Position.Top} />
      <div className="logic-node-top">
        <span className="node-icon">AND</span>
        {node.error && <em className="node-error-dot">!</em>}
      </div>
      <strong>AND</strong>
      <span>全部满足</span>
      <Handle id="source" type="source" position={Position.Bottom} />
    </div>
  );
}

export const AndNode = memo(AndNodeBase);
