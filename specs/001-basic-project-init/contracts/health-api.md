# 接口契约：健康检查

## GET /health

**用途**：验证后端服务自身、远程 MySQL 和远程 Redis 的基础状态。

**认证**：本阶段不要求认证。

**请求参数**：无。

**成功响应示例**

```json
{
  "status": "ok",
  "checkedAt": "2026-07-05T12:00:00+08:00",
  "app": {
    "status": "ok",
    "env": "development"
  },
  "mysql": {
    "status": "ok",
    "message": "connected"
  },
  "redis": {
    "status": "ok",
    "message": "connected"
  }
}
```

**依赖异常响应示例**

```json
{
  "status": "degraded",
  "checkedAt": "2026-07-05T12:00:00+08:00",
  "app": {
    "status": "ok",
    "env": "development"
  },
  "mysql": {
    "status": "error",
    "message": "mysql connection failed: authentication or network error"
  },
  "redis": {
    "status": "ok",
    "message": "connected"
  }
}
```

## 字段规则

- `status`：整体状态。全部可用时为 `ok`；任一依赖不可用时为 `degraded`。
- `checkedAt`：服务端生成的检查时间。
- `app.status`：后端服务状态。
- `app.env`：来自环境变量的运行环境。
- `mysql.status`：MySQL 连接状态。
- `mysql.message`：MySQL 状态摘要。
- `redis.status`：Redis 连接状态。
- `redis.message`：Redis 状态摘要。

## 安全要求

- 响应不得包含 `MYSQL_PASSWORD`、`REDIS_PASSWORD`、完整 DSN、访问令牌或真实密钥。
- 错误摘要应优先表达错误类别，例如配置缺失、网络错误、认证失败或连接超时。
- 本接口用于本地开发诊断，不声明生产级监控或安全审计能力。
