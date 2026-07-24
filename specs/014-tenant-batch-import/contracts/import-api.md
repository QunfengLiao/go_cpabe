# 导入接口契约

所有接口位于当前租户上下文，使用 `AuthRequired`、`TenantRequired` 和租户管理员权限。请求不传 `tenant_id`。

## 模板

- `GET /api/v1/tenant/import/templates/users`
- `GET /api/v1/tenant/import/templates/org-units`

返回对应 `.xlsx` 文件，响应头提供建议文件名和后端配置的大小/行数限制。

## 预校验

- `POST /api/v1/tenant/import/users/validate`
- `POST /api/v1/tenant/import/org-units/validate`

请求为 multipart 文件字段 `file`。响应包含 `batch_id`、`status`、`limits`、`summary` 和 `rows`。每个错误包含 `row_number`、`field`、`code`、`message`。

## 确认

- `POST /api/v1/tenant/import/users/confirm`
- `POST /api/v1/tenant/import/org-units/confirm`

请求 JSON：`{"batch_id":"..."}`。响应包含批次状态和新增、更新、跳过、失败统计。服务端重新验证批次归属、有效期、文件摘要、数据指纹和权限。

确认成功受理时返回 HTTP `202`，状态为 `QUEUED`；同一批次已处于 `QUEUED`、`IMPORTING` 或 `SUCCEEDED` 时返回当前状态，且不重复执行。

## 批次查询与报告

- `GET /api/v1/tenant/import/batches`
- `GET /api/v1/tenant/import/batches/:batchId`
- `GET /api/v1/tenant/import/batches/:batchId/errors`
- `GET /api/v1/tenant/import/batches/:batchId/status`

列表和详情只返回当前租户数据；错误报告为安全处理后的 `.xlsx`，不包含密码。

轻量状态接口只返回 `batch_id`、`status`、`phase`、`processed_count`、`total_count`、`progress_percent`、统计、时间和稳定失败码，不返回 `rows` 或密码摘要，供前端每 1～2 秒轮询。

## 租户成员分页联动

导入完成后使用 `GET /api/v1/tenants/:id/users?page=1&page_size=50` 刷新成员页。响应包含 `users`、`total`、`page` 和 `page_size`；`users` 只包含当前页成员及其角色。页大小由后端限制，客户端不得通过超大 `page_size` 恢复万级全量响应。
