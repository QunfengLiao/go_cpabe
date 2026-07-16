import { describe, expect, it } from "vitest";
import { applyLocalDecryptionRevealTokens, loadLocalDecryptionRevealTokens, removeLocalDecryptionRevealToken, saveLocalDecryptionRevealToken } from "./decryptionRevealTokens";

class MemoryStorage {
  private values = new Map<string, string>();

  getItem(key: string): string | null {
    return this.values.get(key) ?? null;
  }

  setItem(key: string, value: string): void {
    this.values.set(key, value);
  }

  removeItem(key: string): void {
    this.values.delete(key);
  }
}

describe("本地解密 reveal token 持久化", () => {
  it("按账号和租户隔离保存，并在刷新后的文件列表恢复按钮状态", () => {
    const storage = new MemoryStorage();
    saveLocalDecryptionRevealToken(storage, "tenant-a", "user-a", "file-1", "token-a");
    saveLocalDecryptionRevealToken(storage, "tenant-b", "user-a", "file-1", "token-b");

    expect(loadLocalDecryptionRevealTokens(storage, "tenant-a", "user-a")).toEqual({ "file-1": "token-a" });
    expect(loadLocalDecryptionRevealTokens(storage, "tenant-b", "user-a")).toEqual({ "file-1": "token-b" });

    const restored = applyLocalDecryptionRevealTokens(
      [{ id: "file-1", original_filename: "demo.md", plaintext_size: 1, status: "AVAILABLE", created_at: "now" }],
      storage,
      "tenant-a",
      "user-a"
    ) as Array<{ local_decryption_reveal_token?: string }>;

    expect(restored[0].local_decryption_reveal_token).toBe("token-a");
  });

  it("路径失效后清除指定文件 token，不影响同账号其他文件", () => {
    const storage = new MemoryStorage();
    saveLocalDecryptionRevealToken(storage, "tenant-a", "user-a", "file-1", "token-a");
    saveLocalDecryptionRevealToken(storage, "tenant-a", "user-a", "file-2", "token-b");

    removeLocalDecryptionRevealToken(storage, "tenant-a", "user-a", "file-1");

    expect(loadLocalDecryptionRevealTokens(storage, "tenant-a", "user-a")).toEqual({ "file-2": "token-b" });
  });
});
