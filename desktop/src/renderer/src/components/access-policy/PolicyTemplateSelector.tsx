import { templateTree, type PolicyTemplate } from "./tree/types";

export function PolicyTemplateSelector({ templates, dirty, onApplyTemplate }: { templates: PolicyTemplate[]; dirty: boolean; onApplyTemplate: (template: PolicyTemplate) => void }) {
  return (
    <section className="template-selector">
      <h3>策略模板</h3>
      {templates.map((template) => (
        <button
          key={template.id}
          type="button"
          onClick={() => {
            if (dirty && !window.confirm("应用模板会覆盖当前画布，是否继续？")) return;
            if (templateTree(template)) onApplyTemplate(template);
          }}
        >
          <strong>{template.name}</strong>
          <span>{template.description}</span>
        </button>
      ))}
    </section>
  );
}
