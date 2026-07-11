import { attributeCode, attributeName, attributeOperators, attributeTree, attributeType, findAttributeValue, structuredAttributeValues, type PolicyAttribute, type PolicyOperator, type SimpleFlowNode } from "./tree/types";
import { operatorLabel } from "./tree/display";
import { defaultConditionForAttribute } from "./tree/policyModel";

export function AccessTreeConfigPanel({
  node,
  attributes,
  childCount = 0,
  onChange,
  onDelete
}: {
  node?: SimpleFlowNode;
  attributes: PolicyAttribute[];
  childCount?: number;
  onChange: (node: SimpleFlowNode) => void;
  onDelete: (nodeId: string) => void;
}) {
  if (!node) {
    return (
      <aside className="access-tree-config">
        <div className="access-editor-card-title">
          <span>节点配置</span>
          <small>选择画布节点后编辑条件</small>
        </div>
        <div className="empty-config-state">
          <strong>未选择节点</strong>
          <p>点击画布中的 AND、OR 或属性节点后，这里会显示可编辑配置。</p>
        </div>
      </aside>
    );
  }
  const selectedAttr = attributes.find((attr) => attributeCode(attr) === node.data.attribute);
  const selectedType = selectedAttr ? attributeType(selectedAttr) : "string";
  const availableOperators: PolicyOperator[] = selectedAttr ? attributeOperators(selectedAttr) : ["="];
  return (
    <aside className="access-tree-config">
      <div className="access-editor-card-title">
        <span>节点配置</span>
        <small>{node.data.nodeType === "LEAF" ? "属性条件" : `${node.data.nodeType} 逻辑节点`}</small>
      </div>
      <div className={`selected-node-summary selected-node-${node.data.nodeType.toLowerCase()}`}>
        <strong>{node.data.nodeType === "LEAF" ? selectedAttr ? attributeName(selectedAttr) : "未选择属性" : node.data.nodeType}</strong>
        <span>{node.data.nodeType === "LEAF" ? "配置属性、操作符和值" : `当前包含 ${childCount} 个子条件`}</span>
      </div>
      <label className="config-field">
        <span>节点类型</span>
        <select value={node.data.nodeType} onChange={(event) => onChange({ ...node, type: event.target.value === "AND" ? "andNode" : event.target.value === "OR" ? "orNode" : "attributeNode", data: { ...node.data, nodeType: event.target.value as "AND" | "OR" | "LEAF" } })}>
          <option value="AND">且 - 全部满足</option>
          <option value="OR">或 - 任一满足</option>
          <option value="LEAF">属性条件</option>
        </select>
      </label>
      {node.data.nodeType === "LEAF" && (
        <>
          <label className="config-field">
            <span>属性</span>
            <select value={node.data.attribute ?? ""} onChange={(event) => {
              const attr = attributes.find((item) => attributeCode(item) === event.target.value);
              const condition = attr ? defaultConditionForAttribute(attr) : { field: event.target.value, operator: "=" as PolicyOperator, value: "" };
              onChange({ ...node, data: { ...node.data, attribute: condition.field, operator: condition.operator, value: condition.value, valueId: condition.valueId, valueCode: condition.valueCode, label: condition.label, path: condition.path } });
            }}>
              <option value="">请选择</option>
              {attributes.map((attr) => <option key={attributeCode(attr)} value={attributeCode(attr)}>{attributeName(attr)}</option>)}
            </select>
          </label>
          <label className="config-field">
            <span>操作符</span>
            <select value={node.data.operator ?? availableOperators[0] ?? "="} onChange={(event) => onChange({ ...node, data: { ...node.data, operator: event.target.value as PolicyOperator } })}>
              {availableOperators.map((operator) => <option key={operator} value={operator}>{operatorLabel(operator)}</option>)}
            </select>
          </label>
          <label className="config-field">
            <span>属性值</span>
            {selectedAttr && (selectedType === "enum" || selectedType === "tree") ? (
              <select value={String(node.data.value ?? "")} onChange={(event) => {
                const matched = findAttributeValue(selectedAttr, event.target.value);
                onChange({ ...node, data: { ...node.data, value: event.target.value, valueId: matched?.valueId, valueCode: matched?.valueCode, label: matched?.label, path: matched?.path } });
              }}>
                <option value="">请选择</option>
                {valueOptions(selectedAttr).map((option) => <option key={option.valueCode} value={option.valueCode}>{option.label}</option>)}
              </select>
            ) : (
              <input type={selectedAttr && selectedType === "number" ? "number" : "text"} value={String(node.data.value ?? "")} onChange={(event) => onChange({ ...node, data: { ...node.data, value: selectedType === "number" ? Number(event.target.value) : event.target.value } })} />
            )}
          </label>
        </>
      )}
      <button type="button" className="delete-node-button" onClick={() => onDelete(node.id)}>删除节点</button>
    </aside>
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
