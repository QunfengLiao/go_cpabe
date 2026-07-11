import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { FlowAccessNodeData } from "../tree/types";
import { attributeTypeLabel, operatorLabel } from "../tree/display";

function AttributeNodeBase({ data, selected }: NodeProps) {
  const node = data as FlowAccessNodeData;
  const value = typeof node.displayValue === "string" ? node.displayValue : node.value === "" || node.value === undefined || node.value === null ? "未填写" : String(node.value);
  const operation = typeof node.operatorLabel === "string" ? node.operatorLabel : operatorLabel(node.operator);
  return (
    <div className={`access-tree-node attribute-node${selected ? " selected" : ""}${node.error ? " invalid" : ""}`}>
      <Handle id="target" type="target" position={Position.Top} />
      <div className="attribute-node-top">
        <span className="attribute-node-type">{attributeTypeLabel(node.attributeType)}</span>
        {node.error && <em className="node-error-dot">!</em>}
      </div>
      <strong>{String(node.attributeName ?? node.attribute ?? "未选择属性")}</strong>
      <span>{operation} {value}</span>
    </div>
  );
}

export const AttributeNode = memo(AttributeNodeBase);
