export function PolicySettingsModal({
  name,
  description,
  onNameChange,
  onDescriptionChange,
  onClose
}: {
  name: string;
  description: string;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
  onClose: () => void;
}) {
  return (
    <div className="policy-settings-modal-backdrop" role="presentation" onClick={onClose}>
      <section className="policy-settings-modal" role="dialog" aria-modal="true" aria-label="策略设置" onClick={(event) => event.stopPropagation()}>
        <div className="modal-title-row">
          <div>
            <span>策略设置</span>
            <strong>基础信息</strong>
          </div>
          <button type="button" onClick={onClose}>关闭</button>
        </div>
        <label className="config-field">
          <span>策略名称</span>
          <input value={name} onChange={(event) => onNameChange(event.target.value)} />
        </label>
        <label className="config-field">
          <span>策略描述</span>
          <textarea value={description} rows={4} onChange={(event) => onDescriptionChange(event.target.value)} />
        </label>
      </section>
    </div>
  );
}
