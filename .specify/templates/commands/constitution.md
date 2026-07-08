# 宪章命令

根据用户提供的原则输入更新 `.specify/memory/constitution.md`，并同步受影响的模板和运行文档。

必需检查：

- 如存在 `.specify/extensions.yml`，检查宪章更新前后的扩展 hook。
- 保留或初始化 `.specify/memory/constitution.md`。
- 替换所有占位符；如需延期处理，必须显式记录 TODO。
- 应用语义化版本规则。
- 在宪章顶部写入同步影响报告。
- 验证 `.specify/templates/plan-template.md`、`.specify/templates/spec-template.md`、
  `.specify/templates/tasks-template.md` 和 `.specify/templates/commands/*.md`。
- 如存在运行文档，必须同步更新。
- 所有生成的说明性文档必须使用简体中文；代码标识符、路径、命令、API 路径、JSON 字段、
  数据库表名、Go/TypeScript 类型名、第三方库和算法名称除外。
- 如宪章新增 AI 协作、代码注释或可读性原则，必须同步 plan/spec/tasks 模板，并在实现类
  任务中保留关键中文注释和可读性检查。
- 如宪章强化既有代码注释原则，也必须同步 plan/spec/tasks 模板；plan 需要说明注释策略，
  spec 需要声明功能是否触及核心业务代码，tasks 必须生成“关键注释和可读性检查”任务。
- 如宪章强化 Go 注释规范，必须同步要求每个函数/方法有前置注释，导出标识符符合 GoDoc
  前缀规范，并覆盖实体字段、Handler、Service、Repository、Middleware 的业务语义和安全边界。
