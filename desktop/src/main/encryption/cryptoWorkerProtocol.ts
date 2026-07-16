import type { CryptoWorkerProgress, CryptoWorkerResult } from "./types";

export const MAX_WORKER_FRAME_BYTES = 2 * 1024 * 1024;

export type WorkerFrame =
  | { type: "progress"; progress: CryptoWorkerProgress }
  | { type: "result"; result: CryptoWorkerResult }
  | { type: "key_pair"; key_pair: { public_key_pem: string; private_key_pem: string; fingerprint_sha256: string; key_bits: number } }
  | { type: "decryption_result"; decryption: { plaintext_size: number; decrypt_ms: number; key_recovery_ms?: number; file_decryption_ms?: number; plaintext_write_ms?: number } }
  | { type: "error"; error_code: string; message: string };

export function encodeWorkerRequest(value: unknown): Buffer {
  const payload = Buffer.from(JSON.stringify(value), "utf8");
  if (payload.length <= 0 || payload.length > MAX_WORKER_FRAME_BYTES) throw new Error("WORKER_REQUEST_INVALID");
  const header = Buffer.allocUnsafe(4);
  header.writeUInt32BE(payload.length);
  return Buffer.concat([header, payload]);
}

export class WorkerFrameDecoder {
  private buffered = Buffer.alloc(0);

  push(chunk: Buffer): WorkerFrame[] {
    this.buffered = Buffer.concat([this.buffered, chunk]);
    const frames: WorkerFrame[] = [];
    while (this.buffered.length >= 4) {
      const length = this.buffered.readUInt32BE(0);
      if (length <= 0 || length > MAX_WORKER_FRAME_BYTES) throw new Error("WORKER_RESPONSE_INVALID");
      if (this.buffered.length < 4 + length) break;
      const payload = this.buffered.subarray(4, 4 + length);
      this.buffered = this.buffered.subarray(4 + length);
      const value = JSON.parse(payload.toString("utf8")) as WorkerFrame;
      validateWorkerFrame(value);
      frames.push(value);
    }
    return frames;
  }

  finish(): void {
    if (this.buffered.length !== 0) throw new Error("WORKER_RESPONSE_TRUNCATED");
  }
}

function validateWorkerFrame(frame: WorkerFrame): void {
  if (!frame || typeof frame !== "object" || !["progress", "result", "key_pair", "decryption_result", "error"].includes(frame.type)) throw new Error("WORKER_RESPONSE_INVALID");
  const serialized = JSON.stringify(frame).toLowerCase();
  if (frame.type !== "key_pair" && (serialized.includes("private_key") || serialized.includes("source_path") || serialized.includes("output_path") || serialized.includes('"dek"'))) {
    throw new Error("WORKER_RESPONSE_SENSITIVE_FIELD");
  }
}
