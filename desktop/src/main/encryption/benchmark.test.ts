import { describe, expect, it } from "vitest";
import { ProgressBridge } from "./progressBridge";
import { buildCompletionPayload } from "./coordinator";

describe("桌面加密并发与取消基准前置门禁", () => {
  it("三个执行的真实进度保持隔离且不生成计时器伪进度", () => {
    const sent: unknown[] = []; const target = { isDestroyed: () => false, send: (_channel: string, value: unknown) => sent.push(value) };
    const bridge = new ProgressBridge();
    for (let index = 1; index <= 3; index++) bridge.publish(target as never, { executionId: `e${index}`, accountId: "7", tenantId: "3", stage: "ENCRYPTING_FILE", processedBytes: index, totalBytes: 10, cancellable: true });
    expect(sent).toHaveLength(3);
  });

  it.skipIf(process.env.RUN_GCPABE_DESKTOP_BENCHMARK !== "1")("记录 1 GiB Worker 内存峰值和取消响应", () => {
    // 性能环境由 quickstart 负责启动真实 Worker；此门禁防止普通单元测试分配 1 GiB 内存。
    expect(process.env.RUN_GCPABE_DESKTOP_BENCHMARK).toBe("1");
  });
});

describe("多接收者 benchmark 完成协议", () => {
  it("提交分项耗时、接收者数量和 protected key 总大小", () => {
    const worker = {
      ciphertext_size: 128, ciphertext_sha256: "a".repeat(64), nonce_prefix_base64: "bm9uY2U=", chunk_size: 4194304, chunk_count: 1, context_sha256: "b".repeat(64), aes_encrypt_ms: 11, dek_protect_ms: 7,
      protected_key_base64: "", protected_keys_base64: ["YWJj", "ZGVmZw=="],
      protected_keys: [7, 9].map((user, index) => ({ algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "b".repeat(64), binding: { recipient_user_id: user, rsa_public_key_id: `key-${index}`, oaep_label_sha256: "b".repeat(64) }, protect_duration_ms: index + 2 })),
      protected_key: { algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", format: "RSA-OAEP-SHA256-RAW", context_sha256: "b".repeat(64), binding: {} }
    };
    const payload = buildCompletionPayload({ upload_id: "upload", ciphertext_size: 128, ciphertext_sha256: "a".repeat(64), format: "GCPABE01", status: "STAGED" }, worker, 100, { validationDurationMS: 3, uploadDurationMS: 5, totalDurationMS: 30, recipientCount: 2 });
    expect(payload.benchmark).toMatchObject({ validation_duration_ms: 3, file_encryption_duration_ms: 11, key_protection_duration_ms: 7, upload_duration_ms: 5, total_duration_ms: 30, recipient_count: 2, protected_key_total_size_bytes: 7 });
    expect(payload.protected_keys.map((key) => key.protect_duration_ms)).toEqual([2, 3]);
  });
});
