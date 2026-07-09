export function LogicNodeToolbox({ onAddLogic }: { onAddLogic: (type: "AND" | "OR") => void }) {
  return (
    <section className="access-editor-card logic-toolbox">
      <div className="access-editor-card-title">
        <span>逻辑节点</span>
        <small>定义条件组合关系</small>
      </div>
      <div className="logic-toolbox-grid">
        <button type="button" className="logic-tool logic-tool-and" onClick={() => onAddLogic("AND")}>
          <span className="logic-tool-icon">AND</span>
          <strong>新增 AND</strong>
          <small>全部子条件满足</small>
        </button>
        <button type="button" className="logic-tool logic-tool-or" onClick={() => onAddLogic("OR")}>
          <span className="logic-tool-icon">OR</span>
          <strong>新增 OR</strong>
          <small>任一子条件满足</small>
        </button>
      </div>
    </section>
  );
}
