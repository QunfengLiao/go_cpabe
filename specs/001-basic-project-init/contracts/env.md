# 配置契约：环境变量

## `.env.example` 必需字段

```dotenv
APP_ENV=development
APP_PORT=8080

MYSQL_HOST=your-mysql-host
MYSQL_PORT=3306
MYSQL_USER=your-mysql-user
MYSQL_PASSWORD=your-mysql-password
MYSQL_DATABASE=go_cpabe

REDIS_ADDR=your-redis-host:6379
REDIS_PASSWORD=your-redis-password
REDIS_DB=0

DESKTOP_API_BASE_URL=http://localhost:8080
```

## 字段说明

- `APP_ENV`：后端运行环境。
- `APP_PORT`：后端监听端口。
- `MYSQL_HOST`：远程 MySQL 主机。
- `MYSQL_PORT`：远程 MySQL 端口。
- `MYSQL_USER`：远程 MySQL 用户名。
- `MYSQL_PASSWORD`：远程 MySQL 密码。
- `MYSQL_DATABASE`：远程 MySQL 数据库名。
- `REDIS_ADDR`：远程 Redis 地址，格式为 `host:port`。
- `REDIS_PASSWORD`：远程 Redis 密码；如果服务未启用密码，可为空。
- `REDIS_DB`：Redis 逻辑库编号。
- `DESKTOP_API_BASE_URL`：桌面端访问后端的基础地址。

## 安全规则

- `.env.example` 只能包含占位值、说明性示例或本地默认值。
- `.env` 必须被 `.gitignore` 忽略。
- 代码、README、测试和日志不得写死个人服务器 IP、真实数据库密码、真实 Redis 密码或真实令牌。
