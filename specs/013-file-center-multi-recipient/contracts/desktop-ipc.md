# Electron IPC 契约：多接收者加密与本地解密

## 安全原则

- Renderer 只能通过 preload 暴露的受限方法发起加密、解密、选择文件和取消操作。
- Renderer 不得读取 RSA 私钥、明文 DEK、本地完整路径或临时密文路径。
- 主进程不得把 RSA 私钥、明文 DEK、本地完整路径上传给后端。
- 解密成功后主进程可以调用 `shell.showItemInFolder` 打开所在文件夹，但不得直接运行或打开明文文件。

## `desktopEncryption.start(executionId, descriptor)`

启动多接收者加密。

```ts
interface MultiRecipientEncryptionDescriptor {
  accountId: string;
  tenantId: string;
  apiBaseUrl: string;
  accessToken: string;
  idempotencyKey: string;
  fileHandleId: string;
  algorithmCode: string;
  algorithmVersion: string;
  authorization: {
    type: "RSA_RECIPIENTS";
    recipients: Array<{
      recipientUserId: string;
      rsaPublicKeyId: string;
      publicKeyPem: string;
      publicKeyFingerprintSha256: string;
      publicKeyVersion: number;
      owner: boolean;
    }>;
  };
}
```

**行为**：

- 主进程校验 URL、租户、文件句柄、算法和每个公钥 UUID。
- 主进程创建远程任务时只发送接收者用户 ID 和公钥 ID，不发送 PEM。
- Worker 只在本地使用接收者 PEM 保护同一个 DEK。
- 完成请求提交 protected key 数组、接收者绑定数组和性能指标。

## 进度事件

```ts
interface EncryptionProgressEvent {
  executionId: string;
  accountId: string;
  tenantId: string;
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
  uploadedBytes?: number;
  ciphertextBytes?: number;
  protectedRecipients: number;
  totalRecipients: number;
  stageElapsedMs: number;
  totalElapsedMs: number;
  percent: number;
  cancellable: boolean;
}
```

**规则**：

- `ENCRYPTING_FILE` 使用 `processedBytes / totalBytes`。
- `PROTECTING_KEY` 使用 `protectedRecipients / totalRecipients`。
- `UPLOADING` 使用 `uploadedBytes / ciphertextBytes`。
- `SAVING_METADATA` 可显示不确定状态，但不得随机增长。
- `percent` 必须单调递增。

## `desktopEncryption.decryptFile(descriptor)`

执行本地解密。

```ts
interface DecryptFileDescriptor {
  accountId: string;
  tenantId: string;
  apiBaseUrl: string;
  accessToken: string;
  fileId: string;
  suggestedFilename: string;
}

interface DecryptFileResult {
  cancelled: boolean;
  outputFilename?: string;
  outputDirectory?: string;
  revealToken?: string;
}
```

**行为**：

1. 主进程请求当前用户自己的解密材料。
2. 主进程弹出系统原生文件夹选择窗口。
3. 用户取消时返回 `{ cancelled: true }`，不标记远程任务失败。
4. 主进程下载密文到临时目录并校验 SHA-256。
5. 主进程按 `accountId + tenantId + rsaPublicKeyId + fingerprint` 读取本地 RSA 私钥。
6. Worker 解封 DEK 并 AES-GCM 解密到 `.part-*` 临时文件。
7. 成功后将临时文件移动为自动编号的不重名文件。
8. 主进程调用 `shell.showItemInFolder` 打开所在文件夹。
9. 任意失败都清理临时密文和部分明文。

## `desktopEncryption.revealDecryptedFile(revealToken)`

在解密成功提示中兜底打开所在文件夹。

**规则**：

- `revealToken` 只在主进程内映射到最近一次成功输出路径。
- token 不得包含真实路径，不得持久化到服务端。
- token 在账号切换、租户切换或应用退出时清空。

## 账号和租户切换

`encryption:clear-context` 必须：

- 取消活动加密任务；
- 释放文件句柄；
- 清空接收者和文件权限缓存；
- 清空解密 reveal token；
- 清空 RSA 私钥运行时缓存。
