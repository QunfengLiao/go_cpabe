import { createReadStream } from "node:fs";
import { Readable, Transform } from "node:stream";
import { stat } from "node:fs/promises";
import { validateApiBaseUrl } from "./ipcValidation";

interface Envelope<T> { code: string; message: string; data: T; request_id?: string }

export interface RemoteEncryptionTask {
  id: string;
  file_id: string;
  algorithm_code: string;
  algorithm_version: string;
  authorization: Record<string, unknown>;
  authorization_snapshot_sha256?: string;
  status: string;
  failure_code?: string;
  retryable?: boolean;
  current_attempt: { id: string; attempt_no: number; status: string; processed_bytes: number; total_bytes: number; retryable: boolean; failure_code?: string; failure_stage?: string };
}

export interface RemoteTaskLookupEnvelope extends RemoteEncryptionTask {}

export interface CiphertextUploadReceipt {
  upload_id: string;
  ciphertext_size: number;
  ciphertext_sha256: string;
  format: string;
  status: string;
}

export interface CreateEncryptionTaskRequest {
  file: { name: string; size: number; display_mime_type: string };
  algorithm: { code: string; version: string };
  authorization: { type: "RSA_RECIPIENTS"; recipients: Array<{ user_id: number; public_key_id: string }> };
}

export interface CompletionProtectedKey {
  recipient_user_id?: number;
  rsa_public_key_id?: string;
  algorithm_code: string;
  algorithm_version: string;
  format: string;
  value_base64: string;
  context_sha256: string;
  oaep_hash?: "SHA-256";
  oaep_label_sha256?: string;
  protect_duration_ms?: number;
  adapter_binding?: Record<string, unknown>;
}

export interface CompleteEncryptionRequest {
  upload_id: string;
  content_encryption: {
    algorithm: "AES-256-GCM";
    container_format: "GCPABE01";
    encryption_version: "1";
    nonce_prefix_base64: string;
    chunk_size: number;
    chunk_count: number;
    tag_length: 16;
    aad_version: "1";
    context_sha256: string;
  };
  protected_keys: CompletionProtectedKey[];
  benchmark: Record<string, string | number>;
}

export class EncryptionTaskApiClient {
  private readonly baseUrl: string;

  constructor(baseUrl: string, private readonly accessToken: string, private readonly tenantId: string) {
    this.baseUrl = validateApiBaseUrl(baseUrl);
  }

  createTask(idempotencyKey: string, body: CreateEncryptionTaskRequest, signal?: AbortSignal): Promise<RemoteEncryptionTask> {
    return this.json<RemoteEncryptionTask>("/api/v1/tenant/encryption-tasks", { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(body), signal });
  }

  task(taskId: string, signal?: AbortSignal): Promise<RemoteEncryptionTask> {
    return this.json<RemoteEncryptionTask>(`/api/v1/tenant/encryption-tasks/${taskId}`, { method: "GET", signal });
  }

  reportProgress(taskId: string, attemptId: string, body: unknown, signal?: AbortSignal): Promise<RemoteEncryptionTask> {
    return this.json<RemoteEncryptionTask>(`/api/v1/tenant/encryption-tasks/${taskId}/attempts/${attemptId}/progress`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal });
  }

  async uploadCiphertext(taskId: string, attemptId: string, filePath: string, sha256: string, signal?: AbortSignal, onProgress?: (uploadedBytes: number, totalBytes: number) => void): Promise<CiphertextUploadReceipt> {
    const info = await stat(filePath);
    let uploadedBytes = 0;
    const metered = new Transform({
      transform(chunk: Buffer, _encoding, callback) {
        uploadedBytes += chunk.length;
        onProgress?.(uploadedBytes, info.size);
        callback(null, chunk);
      }
    });
    const body = Readable.toWeb(createReadStream(filePath).pipe(metered)) as ReadableStream;
    return this.json<CiphertextUploadReceipt>(`/api/v1/tenant/encryption-tasks/${taskId}/attempts/${attemptId}/ciphertext`, { method: "PUT", headers: { "Content-Type": "application/octet-stream", "Content-Length": String(info.size), "X-Ciphertext-SHA256": sha256, "X-Ciphertext-Format": "GCPABE01" }, body, signal, duplex: "half" } as RequestInit & { duplex: "half" });
  }

  complete(taskId: string, attemptId: string, idempotencyKey: string, body: CompleteEncryptionRequest, signal?: AbortSignal): Promise<unknown> {
    return this.json(`/api/v1/tenant/encryption-tasks/${taskId}/attempts/${attemptId}/complete`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify(body), signal });
  }

  cancel(taskId: string, signal?: AbortSignal): Promise<RemoteEncryptionTask> {
    return this.json<RemoteEncryptionTask>(`/api/v1/tenant/encryption-tasks/${taskId}/cancel`, { method: "POST", signal });
  }

  fail(taskId: string, attemptId: string, failureCode: string, retryable: boolean): Promise<unknown> {
    return this.json(`/api/v1/tenant/encryption-tasks/${taskId}/attempts/${attemptId}/fail`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ failure_code: failureCode, retryable }) });
  }

  private async json<T>(pathname: string, init: RequestInit): Promise<T> {
    const response = await fetch(`${this.baseUrl}${pathname}`, { ...init, headers: { Authorization: `Bearer ${this.accessToken}`, "X-Tenant-Id": this.tenantId, ...(init.headers ?? {}) } });
    const payload = await response.json() as Envelope<T>;
    if (!response.ok || payload.code !== "OK") {
      throw Object.assign(new Error(payload.message || "远程加密请求失败"), {
        code: payload.code || "REMOTE_REQUEST_FAILED",
        status: response.status,
        traceId: payload.request_id || response.headers.get("X-Request-Id") || "",
      });
    }
    return payload.data;
  }
}
