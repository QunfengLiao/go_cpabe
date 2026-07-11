import { AttributeDictionaryPanel } from "./AttributeDictionaryPanel";
import { LogicNodeToolbox } from "./LogicNodeToolbox";
import { PolicyTemplatePanel } from "./PolicyTemplatePanel";
import type { PolicyAttribute, PolicyTemplate } from "./tree/types";

export function AccessTreeSidebar({
  attributes,
  templates,
  dirty,
  attributesLoading = false,
  attributesError = "",
  onAddLogic,
  onCreateAttributeNode,
  onApplyTemplate
}: {
  attributes: PolicyAttribute[];
  templates: PolicyTemplate[];
  dirty: boolean;
  attributesLoading?: boolean;
  attributesError?: string;
  onAddLogic: (type: "AND" | "OR") => void;
  onCreateAttributeNode: (code: string) => void;
  onApplyTemplate: (template: PolicyTemplate) => void;
}) {
  return (
    <aside className="access-tree-sidebar">
      <LogicNodeToolbox onAddLogic={onAddLogic} />
      <AttributeDictionaryPanel attributes={attributes} loading={attributesLoading} error={attributesError} onCreateAttributeNode={onCreateAttributeNode} />
      <PolicyTemplatePanel templates={templates} dirty={dirty} onApplyTemplate={onApplyTemplate} />
    </aside>
  );
}
