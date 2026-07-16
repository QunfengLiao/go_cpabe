# 快速验证：租户管理员新增成员

1. 以租户管理员进入“成员角色”，点击“新增成员”。
2. 输入未注册邮箱，选择 `DU` 并提交。
3. 确认列表出现成员，提示初始密码 `lqf999..`，数据库不含明文密码且 `must_change_password=true`。
4. 用新账号登录并确认页面显示尽快改密提示；登记 RSA 公钥后，由 DO 刷新数据加密页接收者目录。
5. 再用相同邮箱提交 `DO`，确认复用账号、原密码仍有效且不再展示初始密码。
6. 使用普通成员调用接口，确认返回拒绝且没有新增用户、成员、角色和审计成功记录。

## 自动化验证

```powershell
cd D:\04_code\go_cpabe\backend
go test ./internal/service ./internal/handler

cd D:\04_code\go_cpabe\desktop
npm.cmd run typecheck
npm.cmd test
```

## 2026-07-12 实施结果

- Go `go test ./...`：通过。
- Go `go vet ./...`：通过。
- TypeScript `npm.cmd run typecheck`：通过。
- Vitest：27 个测试文件，77 项通过，1 项环境门禁按设计跳过。
- Electron 生产构建：通过，仅保留既有大分块提示。
- 自动化覆盖新账号 bcrypt 初始密码、首次改密、已有账号不重置、非法角色写入前拒绝、当前租户接口和 DU 公钥接收者目录。
