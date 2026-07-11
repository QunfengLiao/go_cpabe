import { attributeCode, attributeName, attributeTree, attributeType, attributeValues, structuredAttributeValues, type PolicyAttribute } from "./tree/types";

export function AttributeDictionaryPanel({
  attributes,
  loading = false,
  error = "",
  onCreateAttributeNode
}: {
  attributes: PolicyAttribute[];
  loading?: boolean;
  error?: string;
  onCreateAttributeNode: (code: string) => void;
}) {
  return (
    <aside className="access-editor-card attribute-panel">
      <div className="access-editor-card-title">
        <span>属性字典</span>
        <small>点击属性创建叶子条件</small>
      </div>
      <div className="attribute-card-list">
        {loading && <div className="attribute-empty-state">正在加载当前租户属性字典...</div>}
        {!loading && error && <div className="attribute-empty-state">{error}</div>}
        {!loading && !error && attributes.length === 0 && <div className="attribute-empty-state">当前租户未初始化属性字典</div>}
        {attributes.map((attribute) => {
          const values = previewValues(attribute);
          return (
            <button key={attributeCode(attribute)} type="button" className="attribute-card" onClick={() => onCreateAttributeNode(attributeCode(attribute))}>
              <span className="attribute-type-tag">{attributeType(attribute)}</span>
              <strong>{attributeName(attribute)}</strong>
              <code>{attributeCode(attribute)}</code>
              {values.length > 0 && <small>{values.slice(0, 3).join(" / ")}{values.length > 3 ? " ..." : ""}</small>}
            </button>
          );
        })}
      </div>
    </aside>
  );
}

function previewValues(attribute: PolicyAttribute): string[] {
  if (attributeType(attribute) === "tree") {
    return attributeTree(attribute).slice(0, 3).map((value) => value.label);
  }
  const structured = structuredAttributeValues(attribute);
  if (structured.length > 0) return structured.slice(0, 3).map((value) => value.label);
  return attributeValues(attribute).slice(0, 3);
}
