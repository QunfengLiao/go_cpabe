# 任务：DU 密钥与本地解密闭环

- [X] T001 完成规格、计划、契约和安全边界文档 `specs/012-du-decryption/`
- [X] T002 [P] [US1] 新增独立密钥管理页面与测试 `desktop/src/renderer/src/pages/MyRSAKeysPage.tsx`
- [X] T003 [US1] 按 `crypto.key.self.manage` 注册密钥菜单和路由 `desktop/src/renderer/src/components/AppLayout.tsx`、`desktop/src/renderer/src/main.tsx`
- [X] T004 [P] [US2] 新增接收者范围 Repository 测试与查询 `backend/internal/repository/encryption_repository.go`
- [X] T005 [US2] 新增收到文件 Service、Handler 和权限路由 `backend/internal/service/encrypted_file_service.go`、`backend/internal/handler/encrypted_file_handler.go`
- [X] T006 [P] [US2] 新增收到文件 API 与页面 `desktop/src/renderer/src/api/encryption.ts`、`desktop/src/renderer/src/pages/ReceivedFilesPage.tsx`
- [X] T007 [P] [US3] 实现 Go Worker RSA+AES 流式解密及篡改测试 `backend/internal/crypto/content_cipher.go`、`backend/cmd/crypto-worker/main.go`
- [X] T008 [US3] 实现历史私钥安全读取 `desktop/src/main/encryption/rsaKeyStore.ts`
- [X] T009 [US3] 实现主进程流式下载、摘要校验、保存对话框和 Worker 调用 `desktop/src/main/encryption/decryptionCoordinator.ts`
- [X] T010 [US3] 接入 IPC/preload 类型并从收到文件页面触发本地解密 `desktop/src/main/main.ts`、`desktop/src/preload/preload.ts`
- [X] T011 [P] 添加跨租户、跨接收者、历史密钥和本地端到端测试
- [X] T012 运行 Go、TypeScript、Vitest 和生产构建全量验证
- [X] T013 执行关键注释、GoDoc、敏感信息和部分明文清理检查
