import { describe, expect, it, vi } from "vitest";
import { copyIntegrityHash, ENCRYPTED_FILE_DETAIL_SECTIONS, resolveRecipients } from "./EncryptedFileDetail";
import { shortHash } from "../../utils/fileDisplay";

describe("加密文件详情抽屉", () => {
  it("提供基本、加密、密钥信封、性能和完整性五组信息", () => {
    expect(ENCRYPTED_FILE_DETAIL_SECTIONS).toEqual(["文件基本信息", "加密信息", "密钥信封接收者", "性能指标", "完整性信息"]);
  });

  it("将统一详情的接收者用户和公钥信息转换为展示模型", () => {
    const recipients = resolveRecipients({ id: "f", original_filename: "demo", plaintext_size: 1, status: "AVAILABLE", created_at: "now" }, {}, [{ user: { id: 9, display_name: "张三", email: "a@example.com" }, public_key_version: 2, fingerprint_sha256: "a".repeat(64), protect_duration_ms: 3 }]);
    expect(recipients).toEqual([expect.objectContaining({ user_id: 9, display_name: "张三", public_key_version: 2, protect_duration_ms: 3 })]);
    expect(shortHash(recipients[0].public_key_fingerprint_sha256)).not.toBe("a".repeat(64));
  });

  it("复制入口只写入完整哈希", async () => {
    const writeText = vi.fn(async () => undefined);
    await copyIntegrityHash("a".repeat(64), { writeText }, () => undefined);
    expect(writeText).toHaveBeenCalledWith("a".repeat(64));
  });
});
