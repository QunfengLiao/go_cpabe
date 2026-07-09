import { attributeCode, attributeName, attributeType, attributeValues, type PolicyAttribute, type PolicyOperator, type SimpleFlowNode } from "./tree/types";

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
          <option value="AND">AND</option>
          <option value="OR">OR</option>
          <option value="LEAF">属性条件</option>
        </select>
      </label>
      {node.data.nodeType === "LEAF" && (
        <>
          <label className="config-field">
            <span>属性</span>
            <select value={node.data.attribute ?? ""} onChange={(event) => onChange({ ...node, data: { ...node.data, attribute: event.target.value, value: "" } })}>
              <option value="">请选择</option>
              {attributes.map((attr) => <option key={attributeCode(attr)} value={attributeCode(attr)}>{attributeName(attr)}</option>)}
            </select>
          </label>
          <label className="config-field">
            <span>操作符</span>
            <select value={node.data.operator ?? "="} onChange={(event) => onChange({ ...node, data: { ...node.data, operator: event.target.value as PolicyOperator } })}>
              <option value="=">=</option>
              <option value="!=">!=</option>
            </select>
          </label>
          <label className="config-field">
            <span>属性值</span>
            {selectedAttr && attributeType(selectedAttr) === "enum" ? (
              <select value={String(node.data.value ?? "")} onChange={(event) => onChange({ ...node, data: { ...node.data, value: event.target.value } })}>
                <option value="">请选择</option>
                {attributeValues(selectedAttr).map((value) => <option key={value} value={value}>{value}</option>)}
              </select>
            ) : (
              <input type={selectedAttr && attributeType(selectedAttr) === "number" ? "number" : "text"} value={String(node.data.value ?? "")} onChange={(event) => onChange({ ...node, data: { ...node.data, value: event.target.value } })} />
            )}
          </label>
        </>
      )}
      <button type="button" className="delete-node-button" onClick={() => onDelete(node.id)}>删除节点</button>
    </aside>
  );
}
