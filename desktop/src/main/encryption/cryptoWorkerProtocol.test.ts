import { describe, expect, it } from "vitest";
import { encodeWorkerRequest, WorkerFrameDecoder } from "./cryptoWorkerProtocol";
import type { CryptoWorkerResult, EncryptionExecutionDescriptor, EncryptionProgressEvent } from "./types";

describe("Crypto Worker 长度前缀协议", () => {
  it("支持拆分读取进度帧", () => {
    const encoded = encodeWorkerRequest({ type: "progress", progress: { stage: "ENCRYPTING_FILE", processed_bytes: 7, total_bytes: 9 } });
    const decoder = new WorkerFrameDecoder();
    expect(decoder.push(encoded.subarray(0, 3))).toEqual([]);
    expect(decoder.push(encoded.subarray(3))).toHaveLength(1);
    expect(() => decoder.finish()).not.toThrow();
  });

  it("拒绝超长和截断帧", () => {
    const header = Buffer.alloc(4); header.writeUInt32BE(2 * 1024 * 1024 + 1);
    expect(() => new WorkerFrameDecoder().push(header)).toThrow("WORKER_RESPONSE_INVALID");
    const decoder = new WorkerFrameDecoder(); decoder.push(encodeWorkerRequest({ type: "error", error_code: "X", message: "x" }).subarray(0, 6));
    expect(() => decoder.finish()).toThrow("WORKER_RESPONSE_TRUNCATED");
  });

  it("类型契约可表达多接收者、protected key 数组和真实阶段进度", () => {
    const descriptor = {
      accountId: "7", tenantId: "3", apiBaseUrl: "http://localhost:18080", accessToken: "token", idempotencyKey: "idem", fileHandleId: "handle",
      algorithmCode: "RSA-OAEP-SHA256", algorithmVersion: "1",
      recipients: [
        { userId: "7", rsaPublicKeyId: "key-1", publicKeyPem: "PUBLIC-1", publicKeyFingerprintSha256: "a".repeat(64) },
        { userId: "9", rsaPublicKeyId: "key-2", publicKeyPem: "PUBLIC-2", publicKeyFingerprintSha256: "b".repeat(64) }
      ],
      authorization: { type: "RSA_RECIPIENT", recipientUserId: "7", rsaPublicKeyId: "key-1", publicKeyPem: "PUBLIC-1", publicKeyFingerprintSha256: "a".repeat(64) }
    } satisfies EncryptionExecutionDescriptor;
    const result = {
      ciphertext_size: 10, ciphertext_sha256: "c".repeat(64), nonce_prefix_base64: "bm9uY2U=", chunk_size: 4194304, chunk_count: 1,
      context_sha256: "d".repeat(64), aes_encrypt_ms: 1, dek_protect_ms: 2, protected_key_base64: "", protected_keys_base64: ["azE=", "azI="],
      protected_keys: descriptor.recipients.map((recipient) => ({ algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "d".repeat(64), binding: { recipient_user_id: Number(recipient.userId), rsa_public_key_id: recipient.rsaPublicKeyId }, protect_duration_ms: 1 })),
      protected_key: { algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "d".repeat(64), binding: {} }
    } satisfies CryptoWorkerResult;
    const progress = { executionId: "e1", accountId: "7", tenantId: "3", stage: "PROTECTING_KEY", processedBytes: 1, totalBytes: 2, protectedRecipients: 1, totalRecipients: 2, stageElapsedMs: 4, totalElapsedMs: 10, percent: 62, cancellable: true } satisfies EncryptionProgressEvent;
    expect(descriptor.recipients).toHaveLength(2);
    expect(result.protected_keys_base64).toHaveLength(2);
    expect(progress.totalRecipients).toBe(2);
  });
});
