import { describe, expect, it } from "vitest";
import { canDownloadEncryptedFile } from "./EncryptedFilesPage";

const base = { id: "file", original_filename: "demo", plaintext_size: 1, created_at: "now" };

describe("加密文件下载可用性", () => {
  it("仅完成可用项允许下载", () => {
    expect(canDownloadEncryptedFile({ ...base, status: "AVAILABLE" })).toBe(true);
    expect(canDownloadEncryptedFile({ ...base, status: "DRAFT" })).toBe(false);
    expect(canDownloadEncryptedFile({ ...base, status: "FAILED" })).toBe(false);
    expect(canDownloadEncryptedFile({ ...base, status: "CANCELLED" })).toBe(false);
  });
});
