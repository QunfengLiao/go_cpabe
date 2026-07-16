import { createDecipheriv, privateDecrypt, constants } from "node:crypto";
import { existsSync } from "node:fs";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { describe, expect, it } from "vitest";
import { encodeWorkerRequest, WorkerFrameDecoder, type WorkerFrame } from "./cryptoWorkerProtocol";

const repoRoot = path.resolve(process.cwd(), "..");
const workerPath = path.join(repoRoot, "backend", "bin", process.platform === "win32" ? "crypto-worker.exe" : "crypto-worker");

describe.skipIf(!existsSync(workerPath))("Go Crypto Worker 本地端到端", () => {
  it("同一密文和同一 DEK 可由多个 RSA 接收者分别解封并还原", async () => {
    const keyFrame = runWorker({ operation: "generate_rsa_key" }).find((frame) => frame.type === "key_pair");
    const secondKeyFrame = runWorker({ operation: "generate_rsa_key" }).find((frame) => frame.type === "key_pair");
    if (!keyFrame || keyFrame.type !== "key_pair" || !secondKeyFrame || secondKeyFrame.type !== "key_pair") throw new Error("missing key pair");
    const directory = await mkdtemp(path.join(os.tmpdir(), "gcpabe-e2e-"));
    const sourcePath = path.join(directory, "plain.bin"); const outputPath = path.join(directory, "cipher.part");
    const plaintext = Buffer.alloc(5 * 1024 * 1024 + 17, 0x5a); await writeFile(sourcePath, plaintext, { mode: 0o600 });
    const authorizations = [keyFrame, secondKeyFrame].map((frame, index) => ({ type: "RSA_RECIPIENT", parameters: { public_key_pem: frame.key_pair.public_key_pem, public_key_fingerprint_sha256: frame.key_pair.fingerprint_sha256, recipient_user_id: 9 + index, rsa_public_key_id: `key-${index + 1}` } }));
    const frames = runWorker({ operation: "encrypt_file", encrypt: { source_path: sourcePath, output_path: outputPath, tenant_id: 3, owner_user_id: 7, task_id: "task", attempt_id: "attempt", file_id: "file", plaintext_size: plaintext.length, algorithm_code: "RSA-OAEP-SHA256", algorithm_version: "1", authorization_snapshot_sha256: "a".repeat(64), authorizations } });
    expect(frames.some((frame) => frame.type === "progress")).toBe(true);
    const resultFrame = frames.find((frame) => frame.type === "result"); if (!resultFrame || resultFrame.type !== "result") throw new Error(`missing result: ${JSON.stringify(frames)}`);
    const container = await readFile(outputPath); const headerLength = container.readUInt32BE(10); const headerBytes = container.subarray(14, 14 + headerLength); const header = JSON.parse(headerBytes.toString("utf8")) as { nonce_prefix_base64: string; chunk_count: number };
    const contextHash = Buffer.from(resultFrame.result.context_sha256, "hex");
    expect(resultFrame.result.protected_keys_base64).toHaveLength(2);
    const dek = privateDecrypt({ key: keyFrame.key_pair.private_key_pem, oaepHash: "sha256", oaepLabel: contextHash, padding: constants.RSA_PKCS1_OAEP_PADDING }, Buffer.from(resultFrame.result.protected_keys_base64![0], "base64"));
    const secondDEK = privateDecrypt({ key: secondKeyFrame.key_pair.private_key_pem, oaepHash: "sha256", oaepLabel: contextHash, padding: constants.RSA_PKCS1_OAEP_PADDING }, Buffer.from(resultFrame.result.protected_keys_base64![1], "base64"));
    expect(secondDEK).toEqual(dek);
    const recovered: Buffer[] = []; let offset = 14 + headerLength; const noncePrefix = Buffer.from(header.nonce_prefix_base64, "base64");
    for (let index = 0; index < header.chunk_count; index++) {
      expect(container.readUInt32BE(offset)).toBe(index); const plainLength = container.readUInt32BE(offset + 4); offset += 8;
      const sealed = container.subarray(offset, offset + plainLength + 16); offset += sealed.length;
      const nonce = Buffer.alloc(12); noncePrefix.copy(nonce); nonce.writeUInt32BE(index, 8);
      const aad = Buffer.alloc(44); contextHash.copy(aad); aad.writeUInt32BE(index, 32); aad.writeUInt32BE(header.chunk_count, 36); aad.writeUInt32BE(plainLength, 40);
      const decipher = createDecipheriv("aes-256-gcm", dek, nonce); decipher.setAAD(aad, { plaintextLength: plainLength }); decipher.setAuthTag(sealed.subarray(sealed.length - 16)); recovered.push(decipher.update(sealed.subarray(0, -16)), decipher.final());
    }
    expect(offset).toBe(container.length); expect(Buffer.concat(recovered)).toEqual(plaintext);
    const restoredPath = path.join(directory, "restored.bin");
    const decryptFrames = runWorker({ operation: "decrypt_file", decrypt: { ciphertext_path: outputPath, output_path: restoredPath, private_key_pem: secondKeyFrame.key_pair.private_key_pem, protected_key_base64: resultFrame.result.protected_keys_base64![1], context_sha256: resultFrame.result.context_sha256, tenant_id: 3, file_id: "file", rsa_public_key_id: "key-2" } });
    expect(decryptFrames.some((frame) => frame.type === "decryption_result")).toBe(true);
    expect(await readFile(restoredPath)).toEqual(plaintext);
  }, 60_000);
});

function runWorker(request: unknown): WorkerFrame[] {
  const execution = spawnSync(workerPath, [], { input: encodeWorkerRequest(request), windowsHide: true, shell: false, maxBuffer: 4 * 1024 * 1024 });
  if (execution.status !== 0) throw new Error("worker failed");
  const decoder = new WorkerFrameDecoder(); const frames = decoder.push(execution.stdout); decoder.finish(); return frames;
}
