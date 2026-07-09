import type { ValidationError } from "./tree/types";

export function PolicyExpressionPreview({ expression, errors }: { expression: string; errors: ValidationError[] }) {
  return (
    <section className={`expression-preview-card${errors.length ? " expression-preview-card-invalid" : ""}`}>
      <div>
        <span>策略表达式</span>
        <strong>{errors.length ? "表达式待修正" : "实时预览"}</strong>
      </div>
      <code>{errors.length ? "请先处理校验结果中的访问树问题" : expression || "从左侧添加节点后生成表达式"}</code>
    </section>
  );
}
