# 数据模型：用户认证与资料基础模块

## 用户 User

### 用途

表示系统账号主体，为后续文件上传、访问策略配置、密钥生成和解密权限判断提供身份基础。

### 字段

| 字段 | 类型建议 | 必填 | 说明 |
|------|----------|------|------|
| `id` | `uint64` 或项目统一 ID 类型 | 是 | 用户 ID，主键 |
| `email` | `string` | 是 | 登录邮箱，唯一 |
| `password_hash` | `string` | 是 | 密码安全哈希，不对外返回 |
| `nickname` | `string` | 是 | 昵称，1 到 20 个字符 |
| `avatar_url` | `string` | 否 | 头像访问地址，对外展示 |
| `avatar_object_key` | `string` | 否 | 头像内部存储标识，不对外返回 |
| `role` | `string` | 是 | `admin`、`data_owner`、`data_user` |
| `status` | `string` | 是 | `active`、`disabled`，默认 `active` |
| `bio` | `string` | 否 | 个人简介，不超过 200 个字符 |
| `birthday` | `date` 或可空日期类型 | 否 | 生日，格式 `YYYY-MM-DD` |
| `created_at` | `datetime` | 是 | 创建时间 |
| `updated_at` | `datetime` | 是 | 更新时间 |
| `deleted_at` | `datetime` | 否 | 软删除时间 |

### Gorm Model 设计

建议在 `internal/domain` 或 `internal/domain/model` 中定义领域模型，并在 Gorm 标签中表达索引、非空、默认值和软删除：

```go
type User struct {
    ID              uint64
    Email           string
    PasswordHash    string
    Nickname        string
    AvatarURL       string
    AvatarObjectKey string
    Role            UserRole
    Status          UserStatus
    Bio             string
    Birthday        *time.Time
    CreatedAt       time.Time
    UpdatedAt       time.Time
    DeletedAt       gorm.DeletedAt
}
```

实施时需要补充 `gorm` 标签：

- `email`：唯一索引，非空，建议长度 255。
- `password_hash`：非空，建议长度 255。
- `nickname`：非空，建议长度 64。
- `role`：非空，建议长度 32。
- `status`：非空，默认 `active`，建议长度 32。
- `bio`：可空或空字符串，建议长度 200。
- `avatar_url` 和 `avatar_object_key`：可空或空字符串，建议长度 512。
- `deleted_at`：启用软删除索引。

### Migration 策略

- 开发环境可以使用 AutoMigrate 快速创建表。
- 项目应同时提供 `migrations/` 下的 SQL migration，作为可审查的数据库结构基准。
- 后续部署、测试环境和 CI 优先执行 SQL migration，避免 AutoMigrate 在受控环境中产生不可预期变更。

### SQL migration 草案

```sql
CREATE TABLE users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  email VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  nickname VARCHAR(64) NOT NULL,
  avatar_url VARCHAR(512) NULL,
  avatar_object_key VARCHAR(512) NULL,
  role VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  bio VARCHAR(200) NULL,
  birthday DATE NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_users_email (email),
  KEY idx_users_deleted_at (deleted_at),
  KEY idx_users_role (role),
  KEY idx_users_status (status)
);
```

### 校验规则

- `email` 必须符合邮箱格式且唯一。
- `password_hash` 不能为空；原始密码不得入库。
- `nickname` 必须为 1 到 20 个字符。
- `role` 只允许 `admin`、`data_owner`、`data_user`；公开注册只允许 `data_owner` 和 `data_user`。
- `status` 只允许 `active` 和 `disabled`。
- `bio` 不超过 200 个字符。
- `birthday` 为空或符合 `YYYY-MM-DD`。

## 用户角色 UserRole

### 常量位置

角色常量放在 `internal/domain`：

- `RoleAdmin = "admin"`
- `RoleDataOwner = "data_owner"`
- `RoleDataUser = "data_user"`

### 规则

- `admin` 本阶段仅预留，公开注册入口禁止创建。
- `data_owner` 可公开注册，后续用于上传文件和设置策略。
- `data_user` 可公开注册，后续用于访问文件和尝试解密。

## 用户状态 UserStatus

### 常量位置

状态常量放在 `internal/domain`：

- `StatusActive = "active"`
- `StatusDisabled = "disabled"`

### 规则

- 新注册用户默认 `active`。
- `disabled` 用户不能登录，后续也不能访问受保护业务入口。

## 用户 DTO

### 用途

所有对外用户响应都使用 `UserDTO`，避免直接序列化数据库模型。

### 字段

| 字段 | 说明 |
|------|------|
| `id` | 用户 ID |
| `email` | 邮箱 |
| `nickname` | 昵称 |
| `role` | 角色 |
| `avatar_url` | 头像访问地址 |
| `bio` | 个人简介 |
| `birthday` | 生日 |
| `status` | 用户状态，注册和登录响应可按需求省略 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间，当前用户资料响应包含 |

### 敏感字段保护

`UserDTO` 不包含：

- `password_hash`
- `avatar_object_key`
- 任何 Token Hash
- Redis 会话内部字段

## Refresh Session

### 用途

表示 Redis 中的 Refresh Token 登录态，用于刷新、退出和后续多端会话扩展。

### Redis Key

```text
auth:refresh:{token_id}
```

### Redis Value

| 字段 | 必填 | 说明 |
|------|------|------|
| `user_id` | 是 | 用户 ID |
| `role` | 是 | 用户角色 |
| `session_id` | 是 | 会话 ID，同一用户多端登录时区分会话 |
| `refresh_token_hash` | 是 | Refresh Token Hash，不保存明文 |
| `issued_at` | 是 | 签发时间 |
| `expires_at` | 是 | 过期时间 |
| `user_agent` | 否 | 客户端用户代理 |
| `client_ip` | 否 | 客户端 IP |

### 状态流转

```text
登录成功 -> 创建 Refresh Session
刷新成功 -> 删除旧 Refresh Session -> 创建新 Refresh Session
退出登录 -> 删除当前 Refresh Session
过期 -> Redis 自动清理
```

## Avatar Object

### 用途

表示头像存储结果，不作为独立数据库表。

### 字段

| 字段 | 说明 |
|------|------|
| `avatar_url` | 前端可访问地址 |
| `avatar_object_key` | 内部存储标识，用于后续删除或迁移 |
| `content_type` | 文件 MIME 类型，可用于校验和响应 |
| `size` | 文件大小，必须不超过 2MB |

### 文件命名

建议格式：

```text
avatars/{user_id}/{timestamp}_{random}.{ext}
```

本地保存路径映射到 `uploads/avatars`，访问地址由头像 URL 前缀和 `avatar_object_key` 拼接生成。
