# Electron 桌面 IPC 契约

## 安全边界

- 文件选择、系统安全存储和上传能力只在 Electron 主进程执行；随机数、DEK 生命周期、AES、RSA 和容器哈希只在固定路径的本地 Go Crypto Worker 执行。
- 预加载脚本通过 `contextBridge` 为每个动作暴露单独方法，不直接暴露 `ipcRenderer`。
- 主进程必须校验 `senderFrame` 属于本应用本地页面，并校验所有对象字段、长度、枚举和标识格式。
- 渲染进程不得获得本地绝对路径、明文 DEK、私钥、临时密文路径或完整受保护密钥。
- API 基地址由主进程配置读取并加入允许列表，渲染进程不得传入任意上传 URL。
- 进度订阅返回移除监听器的函数；回调只接收经过复制和脱敏的普通对象。
- 主进程必须直接启动经过完整性校验的 Crypto Worker 可执行文件，不得通过 shell 或 `PATH` 搜索执行用户可替换程序。

## 暴露对象

```ts
interface DesktopEncryptionAPI {
  selectFile(): Promise<SelectedFile | null>;
  releaseFile(fileHandle: string): Promise<void>;
  ensureRSAKey(input: EnsureRSAKeyInput): Promise<RSAKeyRegistrationMaterial>;
  confirmRSAKeyRegistered(input: ConfirmRSAKeyInput): Promise<void>;
  start(input: StartEncryptionInput): Promise<StartAccepted>;
  cancel(executionId: string): Promise<void>;
  cleanup(executionId: string): Promise<void>;
  onProgress(callback: (event: EncryptionProgressEvent) => void): () => void;
}
```

## `selectFile`

主进程使用原生对话框选择一个文件并建立内存句柄。

```ts
interface SelectedFile {
  fileHandle: string;      // 128 位随机不透明值，仅当前应用会话有效
  name: string;            // 经过展示规范化的文件名
  size: number;            // 1..1 GiB
  displayMimeType?: string;
  lastModifiedMs: number;
}
```

- 取消选择返回 `null`。
- 不返回 `path`。
- 目录、空文件、超过上限、不可读文件立即拒绝。
- 账号或租户切换时，协调器必须释放全部旧文件句柄。

## `ensureRSAKey`

当当前用户在当前租户没有本机可用私钥时，请求本地 Go Crypto Worker 生成 RSA 3072 位密钥对。

```ts
interface EnsureRSAKeyInput {
  accountId: string;
  tenantId: number;
}

interface RSAKeyRegistrationMaterial {
  localKeyHandle: string;
  publicKeyPem: string;
  fingerprintSha256: string;
  keyBits: 3072;
  algorithm: "RSA-OAEP-SHA256";
}
```

- Crypto Worker 返回公钥和短生命周期 PKCS#8 私钥 Buffer；主进程立即使用 Electron 33 支持的同步 `safeStorage.encryptString` 保护私钥，以 `PENDING_REGISTRATION` 状态保存后覆盖临时 Buffer。
- Linux `safeStorage.getSelectedStorageBackend()` 为 `basic_text` 或 `unknown` 时必须拒绝保存。
- 返回对象只含公钥。
- 服务端登记成功后调用 `confirmRSAKeyRegistered`，将服务端公钥 UUID 和版本写入本地索引。
- 登记失败时可用同一公钥重试，不得重复生成大量孤儿私钥。

## `start`

```ts
interface StartEncryptionInput {
  fileHandle: string;
  accountId: string;
  tenantId: number;
  ownerUserId: number;
  accessToken: string;
  task: {
    taskId: string;
    attemptId: string;
    fileId: string;
    totalBytes: number;
  };
  algorithm: {
    code: string;
    version: string;
    authorizationType: string;
  };
  authorization: RSARecipientAuthorization;
}

interface RSARecipientAuthorization {
  type: "RSA_RECIPIENT";
  recipientUserId: number;
  rsaPublicKeyId: string;
  rsaPublicKeyVersion: number;
  publicKeyPem: string;
  publicKeyFingerprintSha256: string;
}

interface StartAccepted {
  executionId: string;
  acceptedAt: number;
}
```

主进程必须校验：

- `fileHandle` 属于当前会话且未被其他执行占用；
- `task.totalBytes` 与重新读取的文件状态一致；
- 算法存在于 Go `CryptoEngine` 能力目录；未知 code 不能回退到 RSA；
- RSA 公钥可解析、位数为 3072且指纹与输入一致；
- `accessToken` 只用于允许列表中的后端请求，不持久化。

主进程随后启动本地 Go Crypto Worker，并通过长度前缀 JSON 控制帧传递执行描述。文件路径、私钥和密钥保存 Buffer 不得进入渲染层事件；Crypto Worker 只返回密文临时文件描述、受保护 DEK、脱敏 Benchmark 和进度，不返回明文 DEK。

## 进度事件

```ts
interface EncryptionProgressEvent {
  executionId: string;
  taskId: string;
  attemptId: string;
  stage:
    | "VALIDATING"
    | "ENCRYPTING_FILE"
    | "PROTECTING_KEY"
    | "UPLOADING"
    | "SAVING_METADATA"
    | "COMPLETED"
    | "FAILED"
    | "CANCELLED";
  processedBytes: number;
  totalBytes: number;
  retryable?: boolean;
  errorCode?: string;
}
```

- 事件不得包含密钥、路径、Authorization header、底层堆栈或受保护密钥。
- `processedBytes` 在同一阶段不得倒退。
- UI 可合并高频事件；服务端进度上报按时间或字节阈值节流。

## 取消

- `cancel` 关闭 Crypto Worker 控制通道并终止本地进程，中止后续读取和上传，同时请求服务端取消当前执行。
- 取消后 Crypto Worker 必须在退出路径覆盖 DEK Buffer、关闭文件描述符；主进程删除 `.part` 文件。
- 若服务端已经进入不可回退的完成事务，取消返回“已完成”或“无法取消”，客户端以服务端状态为准。

## 应用退出和恢复

- 退出时尽力取消运行执行、终止 Crypto Worker 并删除临时密文；不得把 DEK 或 Crypto Worker 内存写入恢复文件。
- 应用启动时扫描本应用命名空间内过期 `.part` 文件并删除。
- 重新登录后通过任务查询把非终态旧执行标记为中断/失败；不自动继续加密或上传。
