# 快速验证：DO 文件加密上传基础闭环

## 目的

本指南用于在实现完成后验证 RSA+AES 首个方案、通用框架边界、租户隔离、失败补偿和敏感信息保护。它不包含完整实现代码；接口细节见 [OpenAPI 契约](./contracts/openapi.yaml)，数据约束见 [数据模型](./data-model.md)。

## 前置条件

- Go 1.23、Node.js 与项目锁文件兼容的 npm、MySQL、Redis。
- Windows 开发机可用 Electron 系统安全存储；Linux 必须确认 `safeStorage.getSelectedStorageBackend()` 不是 `basic_text` 或 `unknown`。当前 Electron 33 使用同步 `safeStorage.encryptString/decryptString`，只允许处理小体积私钥且不得进入高频循环。
- 准备两个租户 A、B；租户 A 至少有一个 DO 和一个 DU，租户 B 至少有一个普通成员。
- DO 拥有 `file.upload`、`file.read`、`file.manage`；DO 和 DU 拥有 `crypto.key.self.manage`；租户管理员拥有 `crypto.key.manage` 和 `audit.read`。
- 后端和桌面端 API 地址必须一致。当前示例中后端默认为 `http://localhost:8080/api/v1`，应把桌面环境变量同步为该值，或同时修改后端监听端口。

建议新增并检查以下配置：

```text
ENCRYPTED_FILE_STORAGE_DIR=uploads/ciphertexts
ENCRYPTED_FILE_TEMP_DIR=uploads/ciphertexts/.staging
ENCRYPTED_FILE_MAX_SIZE=1073741824
ENCRYPTION_MAX_CONCURRENT_PER_TENANT=3
ENCRYPTION_STAGING_TTL=24h
```

密文目录不得注册为 Gin 静态目录。

## 1. 安装与静态检查

```powershell
cd D:\04_code\go_cpabe\backend
go mod download
go test ./...
go build -o ..\desktop\resources\crypto-worker.exe ./cmd/crypto-worker

cd D:\04_code\go_cpabe\desktop
npm ci
npm run typecheck
npm test
npm run build
```

**预期结果**：Go 测试、本地 Crypto Worker 构建、TypeScript 类型检查、Vitest 和 Electron 构建全部通过；编译日志中不出现密钥、Token 或本地文件路径。Electron 只能启动应用资源目录下经过完整性校验的 Crypto Worker，不通过 shell 或 `PATH` 搜索。

## 2. 数据库迁移

复制 `backend/.env.example` 为本地 `.env` 并填写 MySQL、Redis 和 JWT 配置，然后执行：

```powershell
cd D:\04_code\go_cpabe\backend
go run ./cmd/migrate
```

**预期结果**：

- `011_encrypted_file_framework.sql` 可重复执行且后置检查通过。
- 新增算法、公钥、文件、任务、执行、密文对象、受保护密钥、RSA 绑定、Benchmark、孤儿对象和审计表。
- `RSA-OAEP-SHA256` 版本 1 被登记并对演示租户启用。
- 新权限被种子到正确内置角色。
- 重新执行迁移不会重复插入权限、算法或破坏历史数据。

## 3. 启动服务与桌面端

```powershell
cd D:\04_code\go_cpabe\backend
go run ./cmd/server
```

另开终端：

```powershell
cd D:\04_code\go_cpabe\desktop
$env:VITE_API_BASE_URL='http://localhost:8080/api/v1'
npm run dev:electron
```

**预期结果**：服务健康检查正常；桌面端保持 `contextIsolation=true`、`nodeIntegration=false`，数据加密入口只对有权限用户显示。

## 4. 准备 RSA 接收者

1. 使用租户 A 的 DU 登录。
2. 在“我的 RSA 公钥”区域请求本地 Go Crypto Worker 生成密钥。
3. 确认界面只展示公钥指纹、版本和状态，不展示私钥。
4. 退出并检查服务端数据库：仅存在 SPKI 公钥，没有 PKCS#8 私钥。
5. 检查 Electron `userData`：私钥文件内容已通过同步 `safeStorage.encryptString` 保护，不能直接搜索到 PEM 私钥头；Linux 不安全后端必须在生成前被拒绝。

**预期结果**：DU 获得一个 `ACTIVE` 的 3072 位 RSA 公钥版本；重复提交同一幂等键不创建重复版本。

## 5. 主链路验收

1. 使用租户 A 的 DO 登录，进入“数据加密”。
2. 选择一个 1 MiB 至 100 MiB 的普通二进制文件。
3. 确认界面展示名称、大小和类型，但 IPC 返回对象及日志不含完整路径。
4. 选择 `RSA + AES`、租户 A 的 DU 和其有效公钥版本。
5. 发起任务并观察 `VALIDATING → ENCRYPTING_FILE → PROTECTING_KEY → UPLOADING → SAVING_METADATA → COMPLETED`。
6. 在“我的加密文件”查看详情并下载密文。
7. 对下载文件计算 SHA-256，与响应头和详情记录对比。

```powershell
Get-FileHash -Algorithm SHA256 -LiteralPath '<下载的密文路径>'
```

**预期结果**：

- 密文以 `GCPABE01` 开头，大小等于头部、原文长度和 16 字节 tag 之和。
- 列表和详情显示算法、接收者、公钥版本、状态、大小和摘要，不显示对象键或完整受保护 DEK。
- 下载摘要一致，任务与审计记录完整。
- 服务端没有原始明文文件或明文 DEK。

## 6. 密码学自动化验证

运行 Go CryptoEngine 密码学测试和桌面进程边界测试：

```powershell
cd D:\04_code\go_cpabe\backend
go test ./internal/crypto/... ./cmd/crypto-worker/...

cd D:\04_code\go_cpabe\desktop
npm test -- --run cryptoWorker
```

测试必须覆盖：

- 对应测试私钥以 Go `RSAEngine`、RSA-OAEP-SHA-256 和相同 label 恢复 32 字节 DEK。
- 用恢复的 DEK、容器 nonce 前缀、块序号、分块 AAD 和 tag 可还原原始文件。
- 错误私钥、错误 OAEP label、修改头字段、修改/删除/重排/重复分块、追加记录和修改 tag 均失败。
- 同一文件重复加密的 nonce 前缀、密文和受保护 DEK不同。
- 测试 Go Engine 接入时，通用协调器和容器测试无需复制；测试 Engine 不得出现在产品算法列表。

## 7. 幂等与主动重复加密

1. 对同一创建请求并发发送两次相同 `Idempotency-Key`。
2. 验证只返回一个任务和一个首次执行。
3. 对同一源文件选择另一公钥版本，使用新幂等键再次加密。

**预期结果**：网络重试不会产生重复任务；用户主动变更授权会产生可区分的新任务和新密文，不覆盖旧结果。

## 8. 失败补偿

### 上传中断

在 `UPLOADING` 阶段断开网络，再恢复并发起重试。

**预期结果**：旧执行为可重试失败，半成品不可下载；重试创建新执行、新 DEK和新 nonce，最终只有成功对象可下载。

### 数据库完成失败

通过测试故障注入使上传成功后的完成事务失败。

**预期结果**：任务不为 `COMPLETED`；暂存对象被删除，或进入 `orphan_storage_objects` 并由清理命令处理。

```powershell
cd D:\04_code\go_cpabe\backend
go run ./cmd/cleanup
```

### Electron 异常退出

在加密阶段强制退出 Electron，再重新打开。

**预期结果**：旧任务显示中断或失败；启动巡检删除过期 `.part`；恢复数据中不存在 DEK；不会自动继续读取旧文件。

## 9. 租户和权限安全验证

- 用租户 B 的用户、公钥、任务、文件和对象标识替换租户 A 请求中的对应标识。
- 去除 DO 的 `file.upload` 后尝试创建任务。
- 使用 DU 直接调用 DO 创建任务接口。
- 在任务创建前禁用公钥，再提交旧页面缓存中的选择。
- 切换账号或租户后尝试使用旧文件句柄和接收者缓存。

**预期结果**：所有请求在密码学操作或有效业务写入前被拒绝；跨租户资源统一表现为不存在或拒绝；旧文件句柄被释放；产生脱敏安全审计。

## 10. 性能与响应验证

准备 1 GiB 文件并记录：AES 文件加密耗时、RSA DEK 保护耗时、上传耗时、密文大小、进程内存峰值和 UI 操作反馈。

**通过标准**：

- 内存峰值不超过 256 MiB且不随文件大小线性增长。
- 95% 非加密交互在 200 ms 内出现可见反馈，界面不持续无响应超过 1 秒。
- 可安全取消阶段 95% 在 2 秒内停止处理新数据。
- 同租户 3 个并发任务能完成或按配置公平排队；第 4 个得到明确可重试提示。
- 用户和租户请求频率超过令牌桶阈值时返回稳定可重试错误且不创建文件、任务或 DEK；Redis 故障按配置安全失败或回退数据库计数并产生审计。
- 报告分别展示 AES、RSA 和上传指标，不输出“RSA 加密 1 GiB 文件”的误导性结论。

## 11. 敏感信息与注释检查

对数据库、密文目录、Electron 持久文件、服务日志和审计日志执行搜索与人工抽查：

- 不存在 `-----BEGIN PRIVATE KEY-----` 明文。
- 不存在明文 DEK、Authorization header、完整本地路径或完整受保护密钥调试输出。
- 密文存储目录不能经静态 URL 直接访问。
- 所有 Go 函数和方法均有前置中文注释；导出标识符以名称开头；实体字段、Handler、Service、Repository、Middleware、CryptoEngine、Crypto Worker、Storage、Benchmark、Audit 注释说明业务语义、副作用与安全边界。
- 不存在“获取数据”“调用函数”等无意义逐行注释。

## 完成判定

主链路、Go CryptoEngine、幂等补偿、跨租户、安全日志、性能和关键注释检查全部通过，且任何 CP-ABE、访问树秘密共享和 RSA 解密页面未被提前实现，方可认为本功能达到实施验收条件。下一期必须优先评估真实 Go TKN20。

## 2026-07-12 实施验证记录

本次使用经官方 SHA-256 校验的 Go 1.23.12 便携工具链完成 Windows 后端验证；工具链仅位于
系统临时目录，不写入仓库。桌面端使用锁文件现有 Node.js 与 Electron 依赖。

| 验证项 | 命令 | 结果 |
|---|---|---|
| Go 格式化 | `gofmt -w` | 通过 |
| Go 全量测试 | `go test ./...` | 通过；普通回归中 MySQL 隔离库用例在未设置 `TEST_MYSQL_DSN` 时按设计跳过 |
| MySQL 重复迁移 | `TEST_MYSQL_DSN=<隔离库> go test ./internal/migrations -run TestEncryptionFrameworkMigrationAgainstMySQL -count=1 -v` | 通过，临时库中先执行 001–009，再重复执行 010/011 两次并通过 RBAC/加密框架后置校验；临时库已删除 |
| Go 静态检查 | `go vet ./...` | 通过 |
| Go Worker 构建 | `go build -o bin/crypto-worker.exe ./cmd/crypto-worker` | 通过；本机 SHA-256：`4c44efb157a44c991961dcf10deec9d6c1342e47d385393934fcba815b6255cb` |
| 本地 Worker 密码学端到端 | `npm.cmd test -- --run ../main/encryption/encryption.e2e.test.ts` | 通过；真实 3072 位 RSA 私钥恢复 DEK，并还原 5 MiB 多分块原文 |
| TypeScript 类型检查 | `npm.cmd run typecheck` | 通过 |
| Electron 主进程编译 | `npm.cmd run build:main` | 通过 |
| Vitest | `npm.cmd test` | 24 个测试文件，67 项通过；1 项 1 GiB 性能环境门禁按设计跳过 |
| 桌面端生产构建 | `npm.cmd run build` | 通过；仅保留现有 Vite 大分块提示 |
| 16 MiB AES 分块基准 | `go test ./internal/crypto -run '^$' -bench BenchmarkChunkedAES16MiB -benchtime=1x -benchmem` | `40.175 ms/op`、`417.60 MB/s`、约 `20.1 MiB/op`；该结果是 AES 容器组合路径，不解释为 RSA 直接加密文件 |
| 1 GiB 内存门禁 | `RUN_GCPABE_1G_BENCHMARK=1 go test ./internal/crypto -run TestOneGiBMemoryBenchmarkOptIn -count=1 -v` | 通过，耗时 `2.22 s`，堆增长未超过 256 MiB |

本次未修改或清空开发者现有业务库；MySQL 验证使用固定名称隔离临时库，测试后已删除。Redis
三并发租约与第四个稳定拒绝已由 `encryption_admission_test.go` 自动验证。
