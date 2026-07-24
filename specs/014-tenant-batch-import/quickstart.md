# 快速验证：租户数据批量导入

1. 执行 `go run ./cmd/migrate`，应用 `017_tenant_import.sql` 和 `018_async_tenant_import.sql`。
2. 设置 `IMPORT_MAX_FILE_SIZE`、`IMPORT_MAX_ROWS`、`IMPORT_BATCH_TTL`、`IMPORT_WORKER_POLL_INTERVAL`、`IMPORT_WORKER_LEASE` 和 `IMPORT_BULK_SIZE`，启动后端；`IMPORT_MAX_ROWS` 默认支持 10000 行，修改后必须重启后端。
3. 使用租户管理员登录并选择租户，在用户管理或组织管理页点击“批量导入”。
4. 下载对应模板，填写第一张工作表后上传；检查预览统计和逐行错误。
5. 修复错误后重新上传，确认导入；检查用户列表、组织树、成员数量、角色和属性。
6. 在导入记录中查看批次详情，并下载错误报告验证公式前缀已转义。
7. 使用普通租户用户、平台管理员、其他租户批次和过期批次分别验证权限隔离。
8. 导入前准备一行“邮箱已属于其他用户名”的用户，确认预校验返回邮箱占用错误且不能确认。
9. 导入完成后打开成员页，确认默认只返回 50 条、总数正确且翻页不会生成万级 ID 查询。
10. 对包含历史 `user_id=0` 关系的测试库执行迁移，确认五张关系表的零主键关系被清理且真实用户关系不变。

上传页点击“校验并进入预览”后会持续显示处理状态；成功时自动进入数据预览，失败原因保留在当前抽屉中，可修复配置或文件后重试。

验证命令：

```text
cd backend
gofmt -w internal/domain/import.go internal/repository/import_repository.go internal/service/import_service.go internal/handler/import_handler.go
go test ./...
cd ../desktop
npm run typecheck
npm test -- --run
npm run build
```

## 万级异步导入验证

1. 执行迁移并重启后端，确认 Worker 启动。
2. 上传并预校验 10000 行用户模板，点击“确认导入”；接口应在 1 秒内返回 `QUEUED`。
3. 观察页面从“等待处理”进入“正在导入”，进度至少每 2 秒更新一次；关闭并重新打开抽屉后仍能查询同一批次。
4. 连续点击或重复调用确认接口，数据库中只能存在一个执行中的同一批次。
5. 导入成功后检查用户、租户成员、DO/DU 角色和组织成员关系数量；失败场景必须无部分业务写入。
6. 在测试环境处理中止并重启后端，租约到期后任务应被安全重新领取或进入明确失败状态。
