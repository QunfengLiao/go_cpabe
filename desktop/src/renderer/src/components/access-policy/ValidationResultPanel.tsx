import type { ValidationError } from "./tree/types";

export function ValidationResultPanel({ errors }: { errors: ValidationError[] }) {
  if (errors.length === 0) {
    return (
      <section className="validation-panel validation-panel-ok">
        <strong>校验通过</strong>
        <p>当前访问树结构完整，可以保存为访问策略。</p>
      </section>
    );
  }

  return (
    <section className="validation-panel">
      <div className="validation-summary">
        <strong>{errors.length} 个待处理问题</strong>
        <span>这些问题不会阻止继续编辑，但保存前需要修正。</span>
      </div>
      <div className="validation-list">
        {errors.map((error, index) => (
          <article key={`${error.path}-${error.message}-${index}`} className="validation-item">
            <strong>{error.message}</strong>
            <p>{suggestionFor(error.message)}</p>
            <code>{error.path}</code>
          </article>
        ))}
      </div>
    </section>
  );
}

function suggestionFor(message: string): string {
  if (message.includes("至少需要 2 个")) return "请继续添加属性条件，或删除这个暂未完成的逻辑节点。";
  if (message.includes("属性")) return "请在右侧配置面板选择平台开放的属性，并填写有效属性值。";
  if (message.includes("根节点")) return "请保留一个根逻辑节点，并把其他节点连接到它下面。";
  return "请根据节点状态修正访问树配置。";
}
