# 内部契约：审计 Outbox 与 Dispatcher

## 适用范围

本契约描述后端内部审计可靠投递协议，不新增 HTTP、Electron IPC 或桌面端接口。普通业务调用方只能提交类型化、未持久化的审计事件，不能直接写 outbox 状态、租约或正式审计表。

## 入队契约

1. 生成一次稳定 `event_public_id` 和 `occurred_at`。
2. 从认证与租户上下文填充 `tenant_id`、`actor_user_id` 和 `request_id`，不得信任请求体自报值。
3. 根据 `action` 校验允许字段、标量类型、枚举、UUID/SHA-256 格式、数字范围、字符串长度和 JSON 总大小；Metadata 不得携带与顶层可信 `tenant_id` 冲突的副本。
4. Metadata 校验失败时清空 Metadata、设置 `metadata_redacted=true` 并保存顶层最小事件；禁止把原始 Metadata、字段原值、原始错误或敏感值写入 outbox 或普通日志。
5. 安全关键数据库写操作必须使用调用方现有 `*gorm.DB` 事务同时写业务事实和 outbox；入队失败导致事务回滚。
6. 无业务写事务的拒绝、下载和进度事件使用独立短事务入队；MySQL 整体不可用时仅记录不含 Metadata 的结构化告警。

## 领取契约

- 候选状态为到期的 `PENDING`、`RETRY`，以及租约已经过期的 `PROCESSING`。
- 使用 `FOR UPDATE SKIP LOCKED` 和批量上限领取，避免多个 Dispatcher 重复持有同一事件。
- 领取时写入新的随机 `lock_token`、`locked_at` 和 `PROCESSING`。
- 后续成功、失败更新必须同时匹配 `id + PROCESSING + lock_token`；匹配失败表示租约已失效，旧 Worker 不得继续覆盖状态。

## 投递契约

- 使用 `event_public_id` 作为 `audit_logs.public_id`，保留原 `occurred_at`。
- 将 `metadata_redacted` 原样写入 `audit_logs` 的独立字段，不能放回 Metadata JSON。
- `event_public_id` 负责投递幂等；可选 `dedup_key` 负责生产者业务幂等。进度事件的去重摘要必须包含 attempt、stage 和进度版本，避免吞掉合法的多次进度。
- 正式日志插入与 outbox `DELIVERED` 更新在同一 MySQL 事务中完成。
- 正式日志已存在相同 UUID 时视为幂等成功，仍将当前 outbox 收敛为 `DELIVERED`。
- 单条失败不得阻断同批其他事件；失败只保存稳定 `last_error_code`。
- 可重试失败进入 `RETRY`，使用带上限的指数退避和少量抖动；达到最大次数进入 `DEAD_LETTER`。

## 运维契约

- 必须监控待处理最老事件年龄、`DEAD_LETTER` 数量、重试率和投递延迟。
- `DELIVERED` 可按配置保留期清理；`DEAD_LETTER` 不得自动删除。
- 人工重放只能将经过诊断的死信转为 `RETRY`，并记录该管理操作的审计事件。
- 文件孤儿清理命令不得查询、更新或删除 `audit_outbox`。

## 一致性声明

同库事务 outbox 保证 MySQL 可用时安全关键业务事实与待投递审计同时提交，并通过至少一次投递、UUID 幂等达到最终一条正式审计。它不能解决 MySQL 整体不可用；该情况下只能结构化告警，不能宣称审计零丢失。
