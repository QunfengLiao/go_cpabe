# 研究结论：租户数据批量导入

## 现有代码盘点

- `backend/internal/domain/tenant.go` 已有 `TenantUser`、`Role`、`UserRoleAssignment` 和租户成员 DTO。
- `backend/internal/domain/org.go` 已有 `TenantOrgUnit`、`TenantOrgMember`、`TenantOrgMemberRole`，组织树依赖 `parent_id/path/level`。
- `backend/internal/domain/tenant_attribute.go` 已有租户属性、属性值和用户属性投影模型。
- `backend/internal/service/tenant_role_service.go` 与 `backend/internal/repository/rbac_repository.go` 已实现租户角色范围校验、平台角色拒绝和多角色替换。
- `backend/internal/service/org_management_service.go` 已实现组织创建、更新、移动、成员和部门职务事务规则。
- `backend/internal/service/org_attribute_service.go` 与 `org_attribute_repository.go` 已实现组织属性值和用户属性同步。
- `backend/internal/middleware/tenant.go` 从 `X-Tenant-Id` 解析后端可信租户上下文；导入接口使用 `/tenant` 路径并复用该中间件，不在 URL 中接收租户 ID。
- `backend/internal/service/audit_recorder.go` 使用数据库审计 outbox，导入事件使用脱敏白名单和 best-effort 记录。
- `desktop/src/renderer/src/pages/TenantMembersPage.tsx`、`TenantOrgManagementPage.tsx` 是现有用户与组织管理页面，将增加统一导入组件。

## 技术决策

1. 使用 `github.com/xuri/excelize/v2` 读写 `.xlsx`，避免手写 ZIP/XML 解析；服务端先检查扩展名、大小和行数，再把规范化行存入批次。
2. 批次行结果以 JSON 存在 `tenant_import_batch_rows`，并以文件哈希、行数据摘要和当前数据指纹防止预校验结果被篡改或过期使用。
3. 用户和组织分别在独立事务中导入。组织导入使用拓扑排序计算父节点，用户导入批量预取用户、组织、角色和属性索引。
4. 为保持批次原子性，导入服务使用同一个 Gorm 事务执行成员、角色、组织、属性写入；业务规则实现为事务内的私有协作函数，不在 Handler 中直接操作数据库。
5. 模板和错误报告通过内存响应写出；导入临时文件使用随机名并在请求结束时删除，错误报告所有可能被 Excel 解释为公式的字符串加单引号。

## 取舍与风险

- 初始实现受 `IMPORT_MAX_ROWS` 限制并将解析行保存于数据库 JSON，适合演示和中等规模批次；更大规模可迁移至对象存储或临时表。
- 审计采用 best-effort，不让已完成的业务事务因审计下游故障回滚；审计失败会进入现有结构化补偿告警路径。
- 现有单个用户创建接口仍保留旧的响应语义；批量导入不返回初始密码，避免批量响应泄露敏感信息。

## 万级异步导入增量决策

### 决策：使用数据库持久批次作为任务队列

- **理由**：批次本身已经位于 MySQL，使用状态条件、租约令牌和心跳即可支持单实例与多实例领取、进程重启恢复和幂等确认，不需要把 Redis 变成第二个事实来源。
- **替代方案**：请求内 goroutine 无法跨重启恢复；仅使用 Redis 队列需要额外处理数据库与队列的一致性。

### 决策：确认接口快速受理，Worker 执行正式写入

- **理由**：HTTP 请求不再持有长事务，前端能够稳定得到批次编号并查询进度。
- **替代方案**：延长客户端超时只能掩盖问题，不能解决断线、重复点击和服务重启。

### 决策：业务表保持短事务内批量 UPSERT

- **理由**：准备和索引读取在事务前批量完成，事务内按有界大小写入，可将十万级逐行 SQL 降为有限批次，同时保留整批原子性。
- **替代方案**：逐行事务吞吐低；分批直接提交会改变当前“全成或全败”产品语义。

### 决策：状态轮询使用轻量响应

- **理由**：万级行快照约数 MB，进度轮询若重复返回全部行会放大网络、JSON 和渲染开销。
- **替代方案**：WebSocket/SSE 增加桌面端连接管理复杂度；1～2 秒轮询已满足当前管理后台体验。
