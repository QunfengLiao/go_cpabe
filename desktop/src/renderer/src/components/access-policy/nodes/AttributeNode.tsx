import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { FlowAccessNodeData } from "../tree/types";

function AttributeNodeBase({ data, selected }: NodeProps) {
  const node = data as FlowAccessNodeData;
  const value = node.value === "" || node.value === undefined || node.value === null ? "未填写" : String(node.value);
  return (
    <div className={`access-tree-node attribute-node${selected ? " selected" : ""}${node.error ? " invalid" : ""}`}>
      <Handle id="target" type="target" position={Position.Top} />
      <div className="attribute-node-top">
        <span className="attribute-node-type">{String(node.attributeType ?? "attr")}</span>
        {node.error && <em className="node-error-dot">!</em>}
      </div>
      <strong>{String(node.attributeName ?? node.attribute ?? "未选择属性")}</strong>
      <span>{node.operator ?? "="} {value}</span>
    </div>
  );
}

export const AttributeNode = memo(AttributeNodeBase);
