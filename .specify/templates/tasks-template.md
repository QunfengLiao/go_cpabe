# 任务：[FEATURE NAME]

**输入**：来自 `/specs/[FEATURE]/` 的设计文档
**前置条件**：spec.md、plan.md

## 格式：`[ID] [P?] [Area] 任务描述`

- **[P]**：可并行执行
- **[Area]**：Setup、User、File、Policy、Crypto、Benchmark、Audit、Frontend、Test、Docs

## 第 1 阶段：准备

- [ ] T001 [Setup] 验证 backend、frontend 和规格目录结构。
- [ ] T002 [Setup] 新增或更新 MySQL、Redis 和本地开发配置。

## 第 2 阶段：测试优先

- [ ] T003 [P] [Test] 为 spec.md 中描述的成功和失败行为添加测试。
- [ ] T004 [P] [Test] 添加基准断言或测试夹具，确保 AES-GCM 耗时与 DEK 封装/解封装耗时分离。

## 第 3 阶段：核心实现

- [ ] T005 [Crypto] 实现所需 `CryptoEngine` 行为，避免具体算法泄漏到业务服务。
- [ ] T006 [Policy] 实现策略解析、校验、访问树/LSSS 输出和 CP-ABE 库格式转换。
- [ ] T007 [File] 实现文件上传/下载流程，并使用 AES-GCM 加密文件内容。
- [ ] T008 [User] 实现所需用户属性行为。
- [ ] T009 [Benchmark] 记录加密/解密耗时、密文大小和策略复杂度。
- [ ] T010 [Audit] 记录解密成功、解密失败和访问拒绝事件。
- [ ] T011 [Frontend] 实现用户可见工作流和可解释性视图。

## 第 4 阶段：集成

- [ ] T012 [Test] 验证 RSA 和 CP-ABE 流程经过同一业务路径。
- [ ] T013 [Test] 验证策略满足和拒绝原因说明。
- [ ] T014 [Docs] 使用简体中文更新 quickstart 或功能使用说明。

## 第 5 阶段：打磨

- [ ] T015 [Test] 运行完整验证，并在功能记录中写明结果。
