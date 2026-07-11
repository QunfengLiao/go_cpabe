import { useState } from "react";
import { PolicyExpressionPreview } from "./PolicyExpressionPreview";
import { PolicyJsonPreview } from "./PolicyJsonPreview";
import { ValidationResultPanel } from "./ValidationResultPanel";
import type { PolicyAttribute, PolicyTreeNode, ValidationError } from "./tree/types";

type PreviewTab = "expression" | "json" | "validation";

export function PolicyPreviewTabs({
  expression,
  tree,
  attributes = [],
  errors
}: {
  expression: string;
  tree: PolicyTreeNode | null;
  attributes?: PolicyAttribute[];
  errors: ValidationError[];
}) {
  const [activeTab, setActiveTab] = useState<PreviewTab>("expression");
  return (
    <section className="policy-preview-tabs">
      <div className="preview-tab-list" role="tablist" aria-label="策略预览">
        <button type="button" className={activeTab === "expression" ? "active" : ""} onClick={() => setActiveTab("expression")}>表达式预览</button>
        <button type="button" className={activeTab === "json" ? "active" : ""} onClick={() => setActiveTab("json")}>JSON 预览</button>
        <button type="button" className={activeTab === "validation" ? "active" : ""} onClick={() => setActiveTab("validation")}>
          校验结果 {errors.length > 0 ? `(${errors.length})` : ""}
        </button>
      </div>
      <div className="preview-tab-panel">
        {activeTab === "expression" && <PolicyExpressionPreview expression={expression} tree={tree} attributes={attributes} errors={errors} />}
        {activeTab === "json" && <PolicyJsonPreview tree={tree} />}
        {activeTab === "validation" && <ValidationResultPanel errors={errors} />}
      </div>
    </section>
  );
}
