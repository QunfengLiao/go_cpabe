import { attributeCode, attributeName, attributeType, attributeValues, type PolicyAttribute } from "./tree/types";

export function AttributeDictionaryPanel({ attributes, onCreateAttributeNode }: { attributes: PolicyAttribute[]; onCreateAttributeNode: (code: string) => void }) {
  return (
    <aside className="access-editor-card attribute-panel">
      <div className="access-editor-card-title">
        <span>属性字典</span>
        <small>点击属性创建叶子条件</small>
      </div>
      <div className="attribute-card-list">
        {attributes.map((attribute) => {
          const values = attributeValues(attribute);
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
