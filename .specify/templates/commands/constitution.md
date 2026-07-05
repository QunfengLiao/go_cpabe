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
