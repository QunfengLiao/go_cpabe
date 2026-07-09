import type { EditablePolicyTreeNode, PolicyConditionOperator } from "./tree/policyModel";
import type { PolicyAttribute } from "./tree/types";
import { attributeCode, attributeName, attributeType, attributeValues } from "./tree/types";

export function NodeConfigPopover({
  node,
  attributes,
  childCount,
  onChange,
  onDelete
}: {
  node: EditablePolicyTreeNode;
  attributes: PolicyAttribute[];
  childCount: number;
  onChange: (nodeId: string, patch: Partial<EditablePolicyTreeNode>) => void;
  onDelete: (nodeId: string) => void;
}) {
  const selectedAttr = attributes.find((attr) => attributeCode(attr) === node.condition?.field);

  function changeType(nextType: "and" | "or") {
    onChange(node.id, { type: nextType, label: nextType === "and" ? "AND" : "OR" });
  }

  return (
    <section className="node-config-popover" onClick={(event) => event.stopPropagation()}>
      <div className="node-popover-head">
        <strong>{node.type === "attribute" ? "属性条件" : "逻辑节点"}</strong>
        <span>{node.type === "attribute" ? "配置字段和值" : `当前 ${childCount} 个子条件`}</span>
      </div>

      {node.type !== "attribute" && (
        <>
          <label className="config-field">
            <span>节点名称</span>
            <input value={node.label} onChange={(event) => onChange(node.id, { label: event.target.value })} />
          </label>
          <label className="config-field">
            <span>逻辑类型</span>
            <select value={node.type} onChange={(event) => changeType(event.target.value as "and" | "or")}>
              <option value="and">AND - 全部满足</option>
              <option value="or">OR - 任一满足</option>
            </select>
          </label>
        </>
      )}

      {node.type === "attribute" && (
        <>
          <label className="config-field">
            <span>属性字段</span>
            <select value={node.condition?.field ?? ""} onChange={(event) => onChange(node.id, { condition: { field: event.target.value, operator: node.condition?.operator ?? "=", value: "" } })}>
              <option value="">请选择属性</option>
              {attributes.map((attr) => <option key={attributeCode(attr)} value={attributeCode(attr)}>{attributeName(attr)}</option>)}
            </select>
          </label>
          <div className="node-popover-row">
            <label className="config-field">
              <span>操作符</span>
              <select value={node.condition?.operator ?? "="} onChange={(event) => onChange(node.id, { condition: { field: node.condition?.field ?? "", operator: event.target.value as PolicyConditionOperator, value: node.condition?.value ?? "" } })}>
                <option value="=">=</option>
                <option value="!=">!=</option>
                <option value=">">&gt;</option>
                <option value=">=">&gt;=</option>
                <option value="<">&lt;</option>
                <option value="<=">&lt;=</option>
              </select>
            </label>
            <label className="config-field">
              <span>属性类型</span>
              <input value={selectedAttr ? attributeType(selectedAttr) : "-"} disabled />
            </label>
          </div>
          <label className="config-field">
            <span>属性值</span>
            {selectedAttr && attributeType(selectedAttr) === "enum" ? (
              <select value={String(node.condition?.value ?? "")} onChange={(event) => onChange(node.id, { condition: { field: node.condition?.field ?? "", operator: node.condition?.operator ?? "=", value: event.target.value } })}>
                <option value="">请选择值</option>
                {attributeValues(selectedAttr).map((value) => <option key={value} value={value}>{value}</option>)}
              </select>
            ) : (
              <input type={selectedAttr && attributeType(selectedAttr) === "number" ? "number" : "text"} value={String(node.condition?.value ?? "")} onChange={(event) => onChange(node.id, { condition: { field: node.condition?.field ?? "", operator: node.condition?.operator ?? "=", value: event.target.value } })} />
            )}
          </label>
        </>
      )}

      <button type="button" className="delete-node-button" onClick={() => onDelete(node.id)}>删除节点</button>
    </section>
  );
}
