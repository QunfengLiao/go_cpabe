import { describe, expect, it } from "vitest";
import { encodeWorkerRequest, WorkerFrameDecoder } from "./cryptoWorkerProtocol";
import { readFileSync } from "node:fs";
import path from "node:path";
import { access, mkdtemp, writeFile } from "node:fs/promises";
import os from "node:os";
import { decryptFile } from "./decryptionCoordinator";

describe("本地 Worker 响应敏感字段边界", () => {
  it("普通加密结果拒绝私钥和本地路径字段", () => {
    const decoder = new WorkerFrameDecoder();
    expect(() => decoder.push(encodeWorkerRequest({ type: "result", result: { private_key_pem: "secret" } }))).toThrow("WORKER_RESPONSE_SENSITIVE_FIELD");
    expect(() => decoder.push(encodeWorkerRequest({ type: "result", result: { source_path: "C:/secret" } }))).toThrow("WORKER_RESPONSE_SENSITIVE_FIELD");
  });
});

describe("Electron 加密静态安全门禁", () => {
  it("保持隔离、无 shell Worker 和 safeStorage 私钥保护", () => {
    const root = path.resolve(process.cwd(), "src");
    const main = readFileSync(path.join(root, "main", "main.ts"), "utf8");
    const worker = readFileSync(path.join(root, "main", "encryption", "cryptoWorkerProcess.ts"), "utf8");
    const keyStore = readFileSync(path.join(root, "main", "encryption", "rsaKeyStore.ts"), "utf8");
    expect(main).toContain("contextIsolation: true"); expect(main).toContain("nodeIntegration: false");
    expect(worker).toContain("shell: false"); expect(worker).toContain("CRYPTO_WORKER_SHA256");
    expect(keyStore).toContain("safeStorage.encryptString"); expect(keyStore).not.toContain("writeFile(temporary, key.privateKeyPem");
  });
});

describe("本地解密失败清理", () => {
  it("Worker 失败时删除 .part 和临时密文，且不打开文件夹", async () => {
      const outputDirectory = await mkdtemp(path.join(os.tmpdir(), "enc-decrypt-failure-"));
    let partPath = "";
    let ciphertextPath = "";
    let revealCalled = false;
    await expect(decryptFile({ accountId: "7", tenantId: "3", apiBaseUrl: "http://localhost:18080", accessToken: "token", fileId: "123e4567-e89b-42d3-a456-426614174000", suggestedFilename: "demo.txt" }, {
      chooseDirectory: async () => ({ canceled: false, filePaths: [outputDirectory] }),
      fetchMaterial: async () => ({ file_id: "123e4567-e89b-42d3-a456-426614174000", original_filename: "demo.txt", plaintext_size: 5, protected_key_base64: "c2VhbGVk", context_sha256: "a".repeat(64), rsa_public_key_id: "123e4567-e89b-42d3-a456-426614174000", public_key_fingerprint_sha256: "b".repeat(64) }),
      download: async (_base, _file, _headers, target) => { ciphertextPath = target; await writeFile(target, "cipher"); return 6; },
      loadPrivateKey: async () => "PRIVATE KEY",
      createWorker: () => ({ decrypt: async (request: { output_path: string }) => { partPath = request.output_path; await writeFile(partPath, "partial"); throw new Error("RSA_FAILED"); }, terminate: () => undefined }),
      reveal: async () => { revealCalled = true; return true; }
    })).rejects.toThrow("RSA_FAILED");
    await expect(access(partPath)).rejects.toThrow();
    await expect(access(ciphertextPath)).rejects.toThrow();
    expect(revealCalled).toBe(false);
  });
});
