import { access, mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { decryptFile, rememberDecryptedFile, resolveNonOverwritingPath, revealDecryptedFile, safeFilename } from "./decryptionCoordinator";

describe("本地解密输出路径", () => {
  it("清理不可信文件名并保留普通扩展名", () => {
    expect(safeFilename("..\\secret<bad>.txt")).toBe("secret_bad_.txt");
    expect(safeFilename("   ")).toBe("decrypted-file");
  });

  it("同名文件存在时生成编号路径，不静默覆盖", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-decrypt-test-"));
    await writeFile(path.join(directory, "demo.txt"), "existing");
    await writeFile(path.join(directory, "demo (1).txt"), "existing");

    await expect(resolveNonOverwritingPath(directory, "demo.txt")).resolves.toBe(path.join(directory, "demo (2).txt"));
  });

  it("通过主进程 reveal token 打开上次解密位置，不向渲染层暴露完整路径", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-reveal-test-"));
    const decryptedPath = path.join(directory, "demo.txt");
    await writeFile(decryptedPath, "plaintext");

    const token = rememberDecryptedFile(decryptedPath);
    let revealedPath = "";
    const revealed = await revealDecryptedFile(token, (target) => { revealedPath = target; });

    expect(token).not.toContain(directory);
    expect(revealed).toBe(true);
    expect(revealedPath).toBe(decryptedPath);
  });

  it("上次解密文件被移动或删除时不重新解密，只返回不可打开", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-reveal-missing-test-"));
    const decryptedPath = path.join(directory, "missing.txt");
    await writeFile(decryptedPath, "plaintext");
    const token = rememberDecryptedFile(decryptedPath);
    await rm(decryptedPath);

    await expect(revealDecryptedFile(token, () => { throw new Error("should not reveal"); })).resolves.toBe(false);
  });

  it("打开失败不会删除本地下载令牌，文件恢复后仍可再次打开", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-reveal-retain-test-"));
    const decryptedPath = path.join(directory, "demo.enc");
    const token = rememberDecryptedFile(decryptedPath);
    await expect(revealDecryptedFile(token, () => { throw new Error("should not reveal"); })).resolves.toBe(false);
    await writeFile(decryptedPath, "ciphertext");
    let revealedPath = "";
    await expect(revealDecryptedFile(token, (target) => { revealedPath = target; })).resolves.toBe(true);
    expect(revealedPath).toBe(decryptedPath);
    await rm(directory, { recursive: true, force: true });
  });

  it("用户取消目录选择时不请求解密材料，也不标记远程失败", async () => {
    let materialRequested = false;
    const result = await decryptFile(decryptDescriptor(), {
      chooseDirectory: async () => ({ canceled: true, filePaths: [] }),
      fetchMaterial: async () => { materialRequested = true; throw new Error("不应调用"); }
    });
    expect(result).toEqual({ cancelled: true });
    expect(materialRequested).toBe(false);
  });

  it("选择目录后返回自动重命名文件名和不包含路径的 reveal token", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-decrypt-flow-"));
    await writeFile(path.join(directory, "demo.txt"), "existing");
    const result = await decryptFile(decryptDescriptor(), successfulDependencies(directory));
    expect(result).toMatchObject({ cancelled: false, outputFilename: "demo (1).txt" });
    expect(result.revealToken).not.toContain(directory);
    await expect(access(path.join(directory, "demo (1).txt"))).resolves.toBeUndefined();
    await rm(directory, { recursive: true, force: true });
  });

  it("按密钥信封顺序匹配本地私钥，不把服务端角色结果当作解密依据", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-envelope-match-test-"));
    const tried: string[] = [];
    const result = await decryptFile(decryptDescriptor(), {
      chooseDirectory: async () => ({ canceled: false, filePaths: [directory] }),
      fetchMaterial: async () => ({
        ...successfulMaterial(),
        key_envelopes: [
          { key_id: "sealed-1", protected_key_base64: "first", context_sha256: "a".repeat(64), algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", protected_key_format: "RSA-OAEP-SHA256-RAW", rsa_public_key_id: "key-1", public_key_fingerprint_sha256: "b".repeat(64) },
          { key_id: "sealed-2", protected_key_base64: "second", context_sha256: "a".repeat(64), algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", protected_key_format: "RSA-OAEP-SHA256-RAW", rsa_public_key_id: "key-2", public_key_fingerprint_sha256: "c".repeat(64) }
        ]
      }),
      download: successfulDependencies(directory).download,
      loadPrivateKey: async (_account, _tenant, keyId) => { tried.push(keyId); if (keyId === "key-1") throw new Error("LOCAL_RSA_PRIVATE_KEY_NOT_FOUND"); return "PRIVATE KEY 2"; },
      createWorker: () => ({ decrypt: async (request: { protected_key_base64: string; private_key_pem: string; output_path: string }) => { expect(request.protected_key_base64).toBe("second"); expect(request.private_key_pem).toBe("PRIVATE KEY 2"); await writeFile(request.output_path, "plain"); return { plaintextSize: 5, decryptMs: 1, keyRecoveryMs: 1, fileDecryptionMs: 1, plaintextWriteMs: 1 }; }, terminate: () => undefined }),
      reveal: async () => true
    });
    expect(tried).toEqual(["key-1", "key-2"]);
    expect(result.outputFilename).toBe("demo.txt");
    await rm(directory, { recursive: true, force: true });
  });

  it("没有匹配私钥时保留原始密文包且不生成明文", async () => {
    const directory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-envelope-fallback-test-"));
    const failure = await decryptFile(decryptDescriptor(), {
      chooseDirectory: async () => ({ canceled: false, filePaths: [directory] }),
      fetchMaterial: async () => ({ ...successfulMaterial(), key_envelopes: [{ key_id: "sealed", protected_key_base64: "sealed", context_sha256: "a".repeat(64), algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", protected_key_format: "RSA-OAEP-SHA256-RAW", rsa_public_key_id: "missing-key", public_key_fingerprint_sha256: "b".repeat(64) }] }),
      download: successfulDependencies(directory).download,
      loadPrivateKey: async () => { throw new Error("LOCAL_RSA_PRIVATE_KEY_NOT_FOUND"); },
      createWorker: () => { throw new Error("worker must not start"); }
    }).catch((error) => error);
    expect(failure).toMatchObject({ message: "LOCAL_RSA_PRIVATE_KEY_NOT_FOUND", preservedCiphertextFilename: "demo.txt.enc" });
    await expect(access(path.join(directory, "demo.txt.enc"))).resolves.toBeUndefined();
    await expect(access(path.join(directory, "demo.txt"))).rejects.toThrow();
    await rm(directory, { recursive: true, force: true });
  });
});

function decryptDescriptor() {
  return { accountId: "7", tenantId: "3", apiBaseUrl: "http://localhost:18080", accessToken: "token", fileId: "123e4567-e89b-42d3-a456-426614174000", suggestedFilename: "demo.txt" };
}

function successfulMaterial() {
  return { file_id: decryptDescriptor().fileId, original_filename: "demo.txt", plaintext_size: 5, protected_key_base64: "c2VhbGVk", context_sha256: "a".repeat(64), rsa_public_key_id: decryptDescriptor().fileId, public_key_fingerprint_sha256: "b".repeat(64) };
}

function successfulDependencies(directory: string) {
  return {
    chooseDirectory: async () => ({ canceled: false, filePaths: [directory] }),
    fetchMaterial: async () => successfulMaterial(),
    download: async (_base: string, _file: string, _headers: Record<string, string>, target: string) => { await writeFile(target, "cipher"); return 6; },
    loadPrivateKey: async () => "PRIVATE KEY",
    createWorker: () => ({
      decrypt: async (request: { output_path: string }) => { await writeFile(request.output_path, "plain"); return { plaintextSize: 5, decryptMs: 1, keyRecoveryMs: 1, fileDecryptionMs: 1, plaintextWriteMs: 1 }; },
      terminate: () => undefined
    }),
    reveal: async () => true
  };
}
