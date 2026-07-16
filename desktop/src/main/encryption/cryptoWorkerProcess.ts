import { app } from "electron";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { encodeWorkerRequest, WorkerFrameDecoder, type WorkerFrame } from "./cryptoWorkerProtocol";
import { developmentWorkerCandidates } from "./cryptoWorkerPath";
import type { CryptoWorkerProgress, CryptoWorkerResult } from "./types";

export interface GeneratedRSAKeyPair {
  publicKeyPem: string;
  privateKeyPem: string;
  fingerprintSha256: string;
  keyBits: number;
}

export class CryptoWorkerProcess {
  private child: ChildProcessWithoutNullStreams | null = null;

  async encrypt(request: unknown, onProgress: (progress: CryptoWorkerProgress) => void, signal?: AbortSignal): Promise<CryptoWorkerResult> {
    const frame = await this.run({ operation: "encrypt_file", encrypt: request }, onProgress, signal);
    if (frame.type !== "result") throw new Error("WORKER_RESULT_MISSING");
    return frame.result;
  }

  async generateRSAKey(signal?: AbortSignal): Promise<GeneratedRSAKeyPair> {
    const frame = await this.run({ operation: "generate_rsa_key" }, () => undefined, signal);
    if (frame.type !== "key_pair") throw new Error("WORKER_KEY_RESULT_MISSING");
    return { publicKeyPem: frame.key_pair.public_key_pem, privateKeyPem: frame.key_pair.private_key_pem, fingerprintSha256: frame.key_pair.fingerprint_sha256, keyBits: frame.key_pair.key_bits };
  }

  async decrypt(request: unknown, onProgress: (progress: CryptoWorkerProgress) => void, signal?: AbortSignal): Promise<{ plaintextSize: number; decryptMs: number; keyRecoveryMs?: number; fileDecryptionMs?: number; plaintextWriteMs?: number }> {
    const frame = await this.run({ operation: "decrypt_file", decrypt: request }, onProgress, signal);
    if (frame.type !== "decryption_result") throw new Error("WORKER_DECRYPT_RESULT_MISSING");
    return { plaintextSize: frame.decryption.plaintext_size, decryptMs: frame.decryption.decrypt_ms, keyRecoveryMs: frame.decryption.key_recovery_ms, fileDecryptionMs: frame.decryption.file_decryption_ms, plaintextWriteMs: frame.decryption.plaintext_write_ms };
  }

  terminate(): void {
    this.child?.kill();
    this.child = null;
  }

  private async run(request: unknown, onProgress: (progress: CryptoWorkerProgress) => void, signal?: AbortSignal): Promise<WorkerFrame> {
    const workerPath = await resolveVerifiedWorkerPath();
    return new Promise<WorkerFrame>((resolve, reject) => {
      const child = spawn(workerPath, [], { shell: false, windowsHide: true, stdio: ["pipe", "pipe", "pipe"] });
      this.child = child;
      const decoder = new WorkerFrameDecoder();
      let result: WorkerFrame | null = null;
      const abort = () => child.kill();
      signal?.addEventListener("abort", abort, { once: true });
      child.stdout.on("data", (chunk: Buffer) => {
        try {
          for (const frame of decoder.push(chunk)) {
            if (frame.type === "progress") onProgress(frame.progress);
            else if (frame.type === "error") reject(new Error(frame.error_code || "WORKER_OPERATION_FAILED"));
            else result = frame;
          }
        } catch (error) { reject(error); child.kill(); }
      });
      child.stderr.on("data", () => undefined);
      child.on("error", () => reject(new Error("WORKER_START_FAILED")));
      child.on("close", (code) => {
        signal?.removeEventListener("abort", abort);
        this.child = null;
        try { decoder.finish(); } catch (error) { reject(error); return; }
        if (signal?.aborted) { reject(new Error("ENCRYPTION_CANCELLED")); return; }
        if (code !== 0 || !result) { reject(new Error("WORKER_OPERATION_FAILED")); return; }
        resolve(result);
      });
      child.stdin.end(encodeWorkerRequest(request));
    });
  }
}

async function resolveVerifiedWorkerPath(): Promise<string> {
  const executableName = process.platform === "win32" ? "crypto-worker.exe" : "crypto-worker";
  const candidates = app.isPackaged
    ? [path.join(process.resourcesPath, "crypto-worker", executableName)]
    : developmentWorkerCandidates(app.getAppPath(), process.cwd(), executableName);
  let workerPath = "";
  let bytes: Buffer | undefined;
  for (const candidate of candidates) {
    try {
      bytes = await readFile(candidate);
      workerPath = candidate;
      break;
    } catch (error) {
      const code = (error as NodeJS.ErrnoException).code;
      if (code !== "ENOENT" && code !== "ENOTDIR") throw error;
    }
  }
  if (!bytes || !workerPath) throw new Error("WORKER_NOT_FOUND");
  const actual = createHash("sha256").update(bytes).digest("hex");
  const expected = process.env.CRYPTO_WORKER_SHA256?.trim().toLowerCase();
  if (app.isPackaged && (!expected || expected !== actual)) throw new Error("WORKER_INTEGRITY_FAILED");
  if (expected && expected !== actual) throw new Error("WORKER_INTEGRITY_FAILED");
  return workerPath;
}
