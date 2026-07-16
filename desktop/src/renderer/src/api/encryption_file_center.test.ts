import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

describe("统一文件中心 API 契约", () => {
  it("列表、详情、密文下载和当前用户解密材料都使用 tenant/files 路径", () => {
    const source = readFileSync(path.resolve(process.cwd(), "src/renderer/src/api/encryption.ts"), "utf8");
    expect(source).toContain("export function listFileCenterItems");
    expect(source).toContain("export function getFileCenterDetail");
    expect(source).toContain("export async function downloadFileCiphertext");
    expect(source).toContain("export function getOwnDecryptionMaterial");
    expect(source).toContain("/tenant/files/${encodeURIComponent(fileId)}/decryption-material");
    expect(source).toContain("/tenant/files/${encodeURIComponent(file.id)}/ciphertext");
  });
});
