import { templateTree, type PolicyTemplate } from "./tree/types";

export function PolicyTemplatePanel({
  templates,
  dirty,
  onApplyTemplate
}: {
  templates: PolicyTemplate[];
  dirty: boolean;
  onApplyTemplate: (template: PolicyTemplate) => void;
}) {
  return (
    <section className="access-editor-card template-panel">
      <div className="access-editor-card-title">
        <span>策略模板</span>
        <small>快速载入常用访问结构</small>
      </div>
      <div className="template-card-list">
        {templates.map((template) => (
          <article key={template.id} className="template-card">
            <div>
              <strong>{template.name}</strong>
              <p>{template.description || "暂无描述"}</p>
            </div>
            <button
              type="button"
              onClick={() => {
                if (dirty && !window.confirm("使用模板会覆盖当前画布，是否继续？")) return;
                if (templateTree(template)) onApplyTemplate(template);
              }}
            >
              使用模板
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}
