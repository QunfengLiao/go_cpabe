export function AccessTreeToolbar({ dirty, saving, hasErrors, onSave, onValidate, onAutoLayout, onFitView }: {
  dirty: boolean;
  saving: boolean;
  hasErrors: boolean;
  onSave: () => void;
  onValidate: () => void;
  onAutoLayout: () => void;
  onFitView: () => void;
}) {
  return (
    <div className="access-tree-toolbar">
      <button type="button" className="toolbar-secondary" onClick={onFitView}>解析预览</button>
      <button type="button" className="toolbar-secondary" onClick={onAutoLayout}>自动布局</button>
      <button type="button" className="toolbar-secondary" onClick={onValidate}>校验</button>
      <button type="button" className="toolbar-primary" onClick={onSave} disabled={saving || hasErrors}>
        {saving ? "保存中..." : "保存策略"}
      </button>
      <span className={dirty ? "save-state save-state-dirty" : "save-state"}>{dirty ? "未保存" : "已同步"}</span>
    </div>
  );
}
