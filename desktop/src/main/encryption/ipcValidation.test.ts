import { describe, expect, it } from "vitest";
import { assertControlledPath, requireAlgorithm, requireOpaqueFileHandle, requireTenantId, validateApiBaseUrl } from "./ipcValidation";
import { readFileSync } from "node:fs";
import path from "node:path";

describe("加密 IPC 参数校验", () => {
  it("接受受控 UUID、租户和首期算法", () => {
    expect(requireOpaqueFileHandle("123e4567-e89b-42d3-a456-426614174000")).toBeTruthy();
    expect(requireTenantId("7")).toBe("7");
    expect(() => requireAlgorithm("RSA-OAEP-SHA256", "1")).not.toThrow();
  });

  it("拒绝未知算法、路径穿越和任意上传域名", () => {
    expect(() => requireAlgorithm("TKN20-MOCK", "1")).toThrow();
    expect(() => assertControlledPath("C:/outside/file", "C:/safe/root")).toThrow("PATH_REJECTED");
    expect(() => validateApiBaseUrl("https://evil.example")).toThrow("API_BASE_URL_REJECTED");
  });

  it("默认允许本地 18080 后端作为加密 API 来源", () => {
    expect(validateApiBaseUrl("http://localhost:18080/api/v1")).toBe("http://localhost:18080");
    expect(validateApiBaseUrl("http://127.0.0.1:18080/api/v1")).toBe("http://127.0.0.1:18080");
  });

  it("preload 只暴露受限解密与 reveal 调用，不暴露路径或私钥读取能力", () => {
    const preload = readFileSync(path.resolve(process.cwd(), "src/preload/preload.ts"), "utf8");
    expect(preload).toContain("decryptFile:");
    expect(preload).toContain("revealDecryptedFile:");
    expect(preload).not.toContain("getPrivateKey:");
    expect(preload).not.toContain("readLocalPath:");
  });
});
