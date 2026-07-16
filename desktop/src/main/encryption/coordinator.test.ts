import { describe, expect, it } from "vitest";
import { normalizedRecipients, ProgressReportQueue, protectedKeyPayloads, sanitizeExecutionError } from "./coordinator";

describe("进度上报顺序", () => {
  it("上传进度队列清空前不得推进到元数据保存阶段", async () => {
    const events: string[] = [];
    let releaseUpload!: () => void;
    const uploadPending = new Promise<void>((resolve) => {
      releaseUpload = resolve;
    });
    const queue = new ProgressReportQueue();

    queue.enqueue(async () => {
      events.push("upload-start");
      await uploadPending;
      events.push("upload-finished");
    });
    const savingMetadata = queue.drain().then(() => events.push("saving-metadata"));

    await Promise.resolve();
    expect(events).toEqual(["upload-start"]);

    releaseUpload();
    await savingMetadata;
    expect(events).toEqual(["upload-start", "upload-finished", "saving-metadata"]);
  });
});

describe("协调器错误脱敏", () => {
  it("保留稳定业务错误码并附带结构化字段", () => {
    const error = sanitizeExecutionError(Object.assign(new Error("上传阶段失败"), { code: "PROTECTED_KEY_INVALID", status: 400, traceId: "req-1" }), { executionId: "exec-1", taskId: "task-1", stage: "SAVING_METADATA" });
    expect(error).toMatchObject({ code: "PROTECTED_KEY_INVALID", retryable: true, executionId: "exec-1", taskId: "task-1", traceId: "req-1", stage: "SAVING_METADATA", causeCode: "PROTECTED_KEY_INVALID" });
  });

  it("完整性和安全存储错误不可直接重试", () => {
    expect(sanitizeExecutionError(Object.assign(new Error(), { code: "WORKER_INTEGRITY_FAILED" })).retryable).toBe(false);
    expect(sanitizeExecutionError(Object.assign(new Error(), { code: "SAFE_STORAGE_UNAVAILABLE" })).retryable).toBe(false);
  });

  it("本地文件句柄失效必须保留稳定错误码", () => {
    expect(sanitizeExecutionError(new Error("FILE_HANDLE_EXPIRED"))).toMatchObject({ code: "FILE_HANDLE_EXPIRED", retryable: true });
  });

  it("未知底层错误不再强制折叠成通用失败", () => {
    expect(sanitizeExecutionError(Object.assign(new Error("disk path C:/secret"), { code: "RAW_DISK_FAILURE" }))).toMatchObject({ code: "RAW_DISK_FAILURE", causeCode: "RAW_DISK_FAILURE" });
  });

  it("401/403 错误保持原样，不应被当作不确定结果恢复", () => {
    expect(sanitizeExecutionError(Object.assign(new Error("auth expired"), { code: "AUTH_ACCESS_TOKEN_EXPIRED", status: 401 }))).toMatchObject({ code: "AUTH_ACCESS_TOKEN_EXPIRED", causeCode: "AUTH_ACCESS_TOKEN_EXPIRED" });
    expect(sanitizeExecutionError(Object.assign(new Error("forbidden"), { code: "TENANT_PERMISSION_DENIED", status: 403 }))).toMatchObject({ code: "TENANT_PERMISSION_DENIED", causeCode: "TENANT_PERMISSION_DENIED" });
  });
});

describe("多接收者加密契约", () => {
  it("优先使用 descriptor.recipients 并保留旧单接收者兼容", () => {
    const descriptor = {
      accountId: "7",
      tenantId: "3",
      apiBaseUrl: "http://127.0.0.1:8080",
      accessToken: "token",
      idempotencyKey: "idempotency",
      fileHandleId: "file",
      algorithmCode: "RSA-OAEP-SHA256",
      algorithmVersion: "1",
      authorization: { type: "RSA_RECIPIENT", recipientUserId: "9", rsaPublicKeyId: "old-key", publicKeyPem: "OLD", publicKeyFingerprintSha256: "old-fp" },
      recipients: [
        { userId: "7", rsaPublicKeyId: "owner-key", publicKeyPem: "OWNER", publicKeyFingerprintSha256: "owner-fp" },
        { userId: "9", rsaPublicKeyId: "recipient-key", publicKeyPem: "RECIPIENT", publicKeyFingerprintSha256: "recipient-fp" },
      ],
    };

    expect(normalizedRecipients(descriptor)).toHaveLength(2);
    expect(normalizedRecipients({ ...descriptor, recipients: undefined })).toEqual([{ userId: "9", rsaPublicKeyId: "old-key", publicKeyPem: "OLD", publicKeyFingerprintSha256: "old-fp" }]);
  });

  it("把 worker 多份 protected key 转成后端完成请求数组", () => {
    const payloads = protectedKeyPayloads({
      ciphertext_size: 10,
      ciphertext_sha256: "a".repeat(64),
      nonce_prefix_base64: "nonce",
      chunk_size: 4194304,
      chunk_count: 1,
      context_sha256: "d".repeat(64),
      aes_encrypt_ms: 1,
      dek_protect_ms: 2,
      protected_key_base64: "",
      protected_keys_base64: ["first", "second"],
      protected_keys: [
        { algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "d".repeat(64), binding: { recipient_user_id: 7 } },
        { algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "d".repeat(64), binding: { recipient_user_id: 9 } },
      ],
      protected_key: { algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "d".repeat(64), binding: {} },
    });

    expect(payloads).toEqual([
      expect.objectContaining({ value_base64: "first", adapter_binding: { recipient_user_id: 7 } }),
      expect.objectContaining({ value_base64: "second", adapter_binding: { recipient_user_id: 9 } }),
    ]);
  });
});
