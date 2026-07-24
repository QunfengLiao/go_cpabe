# 数据模型：租户数据批量导入

## 导入批次 `tenant_import_batches`

| 字段 | 含义 | 约束 |
|---|---|---|
| id | 内部主键 | 数据库生成 |
| batch_id | 对外批次编号 | UUID，唯一，确认和查询使用 |
| tenant_id | 当前租户 | 必填，所有查询边界 |
| import_type | `users` 或 `org_units` | 必填 |
| file_name | 原始文件名 | 仅展示，不参与路径拼接 |
| file_hash | 原文件 SHA-256 | 确认时校验 |
| rows_json | 规范化行和错误结果 | 仅后端使用，密码字段必须脱敏/清空 |
| total_count/valid_count/success_count/failure_count/skipped_count | 统计 | 后端计算 |
| status | 批次状态 | `UPLOADED/VALIDATING/VALIDATED/IMPORTING/SUCCEEDED/FAILED/EXPIRED` |
| created_by | 操作者 | 当前认证用户 |
| validated_at/confirmed_at/completed_at | 生命周期时间 | 可空 |
| failure_reason | 脱敏失败原因 | 可空，不含原始 SQL/密码 |

## 导入行结果

批次 JSON 中每行包含 `row_number`、`key`、`action`、`status`、`fields` 和 `errors`。`fields` 只保存可安全回显的规范化字段；`initial_password` 永不写入批次 JSON、日志、错误报告或审计。

## 状态约束

- 只有 `VALIDATED` 批次允许确认。
- 确认成功后从 `IMPORTING` 到 `SUCCEEDED`；任何事务错误到 `FAILED`。
- 超过有效期的非完成批次在查询/确认时标为 `EXPIRED`。
- 失败行存在时状态为 `VALIDATED` 但确认接口拒绝；前端默认禁用确认。

## 万级异步执行字段

| 字段 | 含义 | 约束 |
|---|---|---|
| phase | 当前处理阶段 | `QUEUED/PREPARING/WRITING/FINALIZING/COMPLETED` 等稳定枚举 |
| processed_count | 已处理数量 | 0 到 `total_count`，只用于可观察进度 |
| lease_token | Worker 租约令牌 | 可空；领取任务时生成，终态清理 |
| lease_expires_at | 租约到期时间 | 可空；过期的 `IMPORTING` 批次允许重新领取 |
| heartbeat_at | 最近心跳 | 可空；处理期间更新 |
| last_error_code | 稳定失败码 | 可空；不得包含 SQL、密码或跨租户信息 |

新增状态转换：

```text
VALIDATED -> QUEUED -> IMPORTING -> SUCCEEDED
                              \-> FAILED
IMPORTING(租约过期) -> IMPORTING(重新领取)
```

`QUEUED`、`IMPORTING` 和终态重复确认时不得创建新任务。批次进度是运行状态，不作为业务事实；用户、成员、角色、组织和属性仍在最终事务中原子提交。
