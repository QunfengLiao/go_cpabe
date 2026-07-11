import { defaultConditionForAttribute, type EditablePolicyTreeNode, type PolicyConditionOperator } from "./tree/policyModel";
import type { PolicyAttribute } from "./tree/types";
import { attributeCode, attributeName, attributeOperators, attributeTree, attributeType, findAttributeValue, structuredAttributeValues } from "./tree/types";
import { attributeTypeLabel, operatorLabel } from "./tree/display";

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
  const selectedType = selectedAttr ? attributeType(selectedAttr) : "string";
  const availableOperators: PolicyConditionOperator[] = selectedAttr ? attributeOperators(selectedAttr) : ["="];

  function changeType(nextType: "and" | "or") {
    onChange(node.id, { type: nextType, label: nextType === "and" ? "全部满足" : "任一满足" });
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
              <option value="and">且 - 全部满足</option>
              <option value="or">或 - 任一满足</option>
            </select>
          </label>
        </>
      )}

      {node.type === "attribute" && (
        <>
          <label className="config-field">
            <span>属性字段</span>
            <select value={node.condition?.field ?? ""} onChange={(event) => {
              const attr = attributes.find((item) => attributeCode(item) === event.target.value);
              onChange(node.id, { label: attr ? attributeName(attr) : "属性条件", condition: attr ? defaultConditionForAttribute(attr) : { field: event.target.value, operator: "=", value: "" } });
            }}>
              <option value="">请选择属性</option>
              {attributes.map((attr) => <option key={attributeCode(attr)} value={attributeCode(attr)}>{attributeName(attr)}</option>)}
            </select>
          </label>
          <div className="node-popover-row">
            <label className="config-field">
              <span>操作符</span>
              <select value={node.condition?.operator ?? availableOperators[0] ?? "="} onChange={(event) => onChange(node.id, { condition: { ...node.condition, field: node.condition?.field ?? "", operator: event.target.value as PolicyConditionOperator, value: node.condition?.value ?? "" } })}>
                {availableOperators.map((operator) => <option key={operator} value={operator}>{operatorLabel(operator)}</option>)}
              </select>
            </label>
            <label className="config-field">
              <span>属性类型</span>
              <input value={selectedAttr ? attributeTypeLabel(selectedType) : "-"} disabled />
            </label>
          </div>
          <label className="config-field">
            <span>属性值</span>
            {selectedAttr && (selectedType === "enum" || selectedType === "tree") ? (
              <select value={String(node.condition?.value ?? "")} onChange={(event) => {
                const matched = findAttributeValue(selectedAttr, event.target.value);
                onChange(node.id, { condition: { field: node.condition?.field ?? "", operator: node.condition?.operator ?? availableOperators[0] ?? "=", value: event.target.value, valueId: matched?.valueId, valueCode: matched?.valueCode, label: matched?.label, path: matched?.path } });
              }}>
                <option value="">请选择值</option>
                {valueOptions(selectedAttr).map((option) => <option key={option.valueCode} value={option.valueCode}>{option.label}</option>)}
              </select>
            ) : (
              <input type={selectedAttr && selectedType === "number" ? "number" : "text"} value={String(node.condition?.value ?? "")} onChange={(event) => onChange(node.id, { condition: { field: node.condition?.field ?? "", operator: node.condition?.operator ?? availableOperators[0] ?? "=", value: selectedType === "number" ? Number(event.target.value) : event.target.value } })} />
            )}
          </label>
        </>
      )}

      <button type="button" className="delete-node-button" onClick={() => onDelete(node.id)}>删除节点</button>
    </section>
  );
}

function valueOptions(attribute: PolicyAttribute): Array<{ valueCode: string; label: string }> {
  if (attributeType(attribute) === "tree") return flattenTree(attributeTree(attribute), 0);
  return structuredAttributeValues(attribute).map((value) => ({ valueCode: value.valueCode, label: value.label }));
}

function flattenTree(values: ReturnType<typeof attributeTree>, depth: number): Array<{ valueCode: string; label: string }> {
  return values.flatMap((value) => [
    { valueCode: value.valueCode, label: `${"　".repeat(depth)}${value.label}` },
    ...flattenTree(value.children ?? [], depth + 1)
  ]);
}
