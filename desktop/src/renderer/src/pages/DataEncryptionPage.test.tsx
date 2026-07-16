import { describe, expect, it } from "vitest";
import { canStartEncryption, encryptionErrorCode, ENCRYPTION_SUBMIT_TEXT, ensureOwnerSelected, selectedRecipientDescriptors } from "./DataEncryptionPage";
import { confirmationRecipientSummary } from "../components/encryption/EncryptionConfirmation";

const file = { handleId: "handle", name: "demo", size: 1, displayMimeType: "application/octet-stream", lastModifiedMs: 1 };
const key = { id: "key", user_id: 9, version: 1, fingerprint_sha256: "a".repeat(64), public_key_pem: "PUBLIC", key_bits: 3072, algorithm: "RSA-OAEP-SHA256", status: "ACTIVE", created_at: "now" };

describe("数据加密提交校验", () => {
  it("要求文件、动态算法、接收者和公钥且执行中禁用", () => {
    expect(canStartEncryption({ file, algorithmCode: "RSA-OAEP-SHA256", selectedKeys: [key], busy: false })).toBe(true);
    expect(canStartEncryption({ file: null, algorithmCode: "RSA-OAEP-SHA256", selectedKeys: [key], busy: false })).toBe(false);
    expect(canStartEncryption({ file, algorithmCode: "RSA-OAEP-SHA256", selectedKeys: [key], busy: true })).toBe(false);
    expect(canStartEncryption({ file, algorithmCode: "RSA-OAEP-SHA256", selectedKeys: [], busy: false })).toBe(false);
  });

  it("从 Electron IPC 错误消息中还原稳定错误码", () => {
    const error = new Error("Error invoking remote method 'encryption:start': Error: FILE_HANDLE_EXPIRED");
    expect(encryptionErrorCode(error)).toBe("FILE_HANDLE_EXPIRED");
  });

  it("把多选 DU 公钥转换为主进程 recipients descriptor", () => {
    const descriptors = selectedRecipientDescriptors([{ userId: 7, key }, { userId: 9, key: { ...key, id: "key-2" } }]);
    expect(descriptors).toEqual([
      expect.objectContaining({ userId: "7", rsaPublicKeyId: "key" }),
      expect.objectContaining({ userId: "9", rsaPublicKeyId: "key-2" })
    ]);
  });

  it("owner 始终锁定在接收者快照中，确认动作使用明确按钮文案", () => {
    const recipients = [
      { user_id: 7, display_name: "拥有者", available: true, active_key_count: 1 },
      { user_id: 9, display_name: "接收者", available: true, active_key_count: 1 }
    ];
    expect(ensureOwnerSelected([9], 7, recipients)).toEqual([7, 9]);
    expect(ensureOwnerSelected([], 7, recipients)).toEqual([7]);
    expect(ENCRYPTION_SUBMIT_TEXT).toBe("开始加密并上传");
    expect(confirmationRecipientSummary([
      { userId: 7, recipient: recipients[0], key },
      { userId: 9, recipient: recipients[1], key: { ...key, id: "key-2", version: 2 } }
    ], 7)).toMatchObject({ count: 2, ownerName: "拥有者", names: ["拥有者", "接收者"], keyVersions: [1, 2] });
  });
});
