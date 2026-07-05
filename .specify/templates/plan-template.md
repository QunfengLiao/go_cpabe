# 实现计划：[FEATURE]

**分支**：`[BRANCH_NAME]` | **日期**：[DATE] | **规格**：[LINK]
**输入**：来自 `/specs/[FEATURE]/spec.md` 的功能规格

## 摘要

[从功能规格中提取：核心需求 + 技术方案]

## 技术上下文

**语言/版本**：Go、TypeScript
**主要依赖**：Gin、Gorm、Electron、MySQL、Redis；涉及 CP-ABE 时优先使用 Cloudflare CIRCL TKN20
**存储**：MySQL 保存元数据和审计记录；Redis 用于缓存或会话需求
**测试**：后端使用 Go 测试；引入桌面端交互时补充 Electron/TypeScript 测试
**目标项目**：Electron 桌面端 + Go 后端的 CP-ABE 加密文件共享系统
**性能目标**：单独记录 AES-GCM 耗时，并与 DEK 封装/解封装耗时分离
**约束**：只允许混合加密；CP-ABE 必须来自真实 Go 库；不得使用模拟密码学结果
**规模/范围**：优先完成单授权机构 MVP，再考虑高级 ABE 能力

## 宪章检查

*关卡：必须在第 0 阶段研究前通过；第 1 阶段设计后必须再次检查。*

- 混合加密：文件内容使用 AES-GCM；RSA/CP-ABE 只封装或解封装 DEK。
- 真实 CP-ABE：CP-ABE 行为来自真实 Go CP-ABE 库，优先评估 CIRCL TKN20。
- 可插拔算法：业务代码依赖 `CryptoEngine`，不直接依赖具体 RSA/TKN20 代码。
- 公平基准：AES-GCM 耗时与 DEK 封装/解封装耗时分离，结论限定适用场景。
- 策略解耦：访问树和 LSSS 用于解释策略，不实现 CP-ABE 密码学逻辑。
- 可解释性：展示成功/失败原因、匹配属性、策略结构和性能指标。
- 模块边界：User、File、Policy、Crypto、Benchmark、Audit 职责保持分离。
- 范围纪律：主链路 MVP 优先于用户撤销、多授权机构 ABE、策略隐藏、区块链审计、
  分布式存储或生产级 KMS。
- 语言规范：本计划以及后续 `research.md`、`data-model.md`、`quickstart.md`、接口说明
  文档必须使用简体中文；代码标识符和工程名称除外。
- AI 注释规范：核心业务代码必须包含必要中文注释；安全、认证、权限、Token、密码、
  文件上传、加密算法和访问控制逻辑必须说明关键设计原因和安全边界；实现后必须进行
  关键注释和可读性检查。

## 项目结构

```text
backend/
frontend/
specs/[FEATURE]/
```

## 第 0 阶段：研究

[记录未知项、库/API 评估、性能基准假设和技术取舍]

## 第 1 阶段：设计

[记录数据模型、接口契约、模块边界和 quickstart 更新]

## 第 2 阶段：任务规划

[由 tasks 工作流生成]

## 复杂度跟踪

[仅在宪章检查存在已论证的偏离时填写]
