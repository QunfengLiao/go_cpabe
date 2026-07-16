import { app, type WebContents } from "electron";
import { randomUUID } from "node:crypto";
import path from "node:path";
import { CryptoWorkerProcess } from "./cryptoWorkerProcess";
import { releaseFileHandle, resolveFileHandle } from "./fileSelection";
import { ProgressBridge } from "./progressBridge";
import { EncryptionTaskApiClient, type CiphertextUploadReceipt, type CompleteEncryptionRequest, type CompletionProtectedKey, type RemoteEncryptionTask } from "./taskApiClient";
import { createTempCiphertextPath, removeTempCiphertext } from "./tempCiphertext";
import { recordActiveExecution, removeActiveExecution } from "./recovery";
import type { CryptoWorkerResult, EncryptionExecutionDescriptor, EncryptionProgressEvent, EncryptionRecipientDescriptor, SanitizedEncryptionError } from "./types";

interface ActiveExecution { controller: AbortController; worker: CryptoWorkerProcess; task?: RemoteEncryptionTask; descriptor: EncryptionExecutionDescriptor; tempPath?: string }

// ProgressReportQueue 串行执行进度请求，并允许阶段切换等待已排队请求全部结束。
export class ProgressReportQueue {
  private tail: Promise<void> = Promise.resolve();

  enqueue(report: () => Promise<void>): void {
    this.tail = this.tail.then(report).catch(() => undefined);
  }

  async drain(): Promise<void> {
    await this.tail;
  }
}

export class EncryptionCoordinator {
  private readonly active = new Map<string, ActiveExecution>();
  private readonly progress = new ProgressBridge();

  async start(target: WebContents, descriptor: EncryptionExecutionDescriptor, suppliedExecutionId?: string): Promise<{ executionId: string; file: unknown }> {
	const totalStarted = performance.now();
	const executionId = suppliedExecutionId ?? randomUUID();
    const controller = new AbortController();
    const worker = new CryptoWorkerProcess();
    const current: ActiveExecution = { controller, worker, descriptor };
    this.active.set(executionId, current);
    const client = new EncryptionTaskApiClient(descriptor.apiBaseUrl, descriptor.accessToken, descriptor.tenantId);
    const handle = resolveFileHandle(descriptor.fileHandleId);
    try {
      this.publish(target, executionId, descriptor, "VALIDATING", 0, handle.size, true);
      const validationStarted = performance.now();
      const recipients = normalizedRecipients(descriptor);
      const task = await client.createTask(descriptor.idempotencyKey, { file: { name: handle.name, size: handle.size, display_mime_type: handle.displayMimeType }, algorithm: { code: descriptor.algorithmCode, version: descriptor.algorithmVersion }, authorization: { type: "RSA_RECIPIENTS", recipients: recipients.map((recipient) => ({ user_id: Number(recipient.userId), public_key_id: recipient.rsaPublicKeyId })) } }, controller.signal);
      current.task = task;
	  const validationDurationMS = elapsedMillis(validationStarted);
	  await recordActiveExecution({ executionId, accountId: descriptor.accountId, tenantId: descriptor.tenantId, taskId: task.id, attemptId: task.current_attempt.id, updatedAt: new Date().toISOString() });
	  await bestEffortProgress(client, task.id, task.current_attempt.id, { stage: "VALIDATING", processed_bytes: 0, total_bytes: handle.size }, controller.signal);
      const tempRoot = path.join(app.getPath("temp"), "go-cpabe", "ciphertexts");
      current.tempPath = await createTempCiphertextPath(tempRoot);
      let lastProgressAt = 0;
      const progressReports = new ProgressReportQueue();
      const workerResult = await worker.encrypt({ source_path: handle.absolutePath, output_path: current.tempPath, tenant_id: Number(descriptor.tenantId), owner_user_id: Number(descriptor.accountId), task_id: task.id, attempt_id: task.current_attempt.id, file_id: task.file_id, plaintext_size: handle.size, algorithm_code: descriptor.algorithmCode, algorithm_version: descriptor.algorithmVersion, authorization_snapshot_sha256: task.authorization_snapshot_sha256 ?? "", authorizations: recipients.map((recipient) => ({ type: "RSA_RECIPIENT", parameters: { public_key_pem: recipient.publicKeyPem, recipient_user_id: Number(recipient.userId), rsa_public_key_id: recipient.rsaPublicKeyId, public_key_fingerprint_sha256: recipient.publicKeyFingerprintSha256 } })) }, (reported) => {
        this.publish(target, executionId, descriptor, reported.stage, reported.processed_bytes, reported.total_bytes, true);
        if (Date.now() - lastProgressAt >= 500) {
          lastProgressAt = Date.now();
          progressReports.enqueue(() => bestEffortProgress(client, task.id, task.current_attempt.id, { stage: reported.stage, processed_bytes: reported.processed_bytes, total_bytes: reported.total_bytes }, controller.signal));
        }
      }, controller.signal);
	  await progressReports.drain();
	  // Worker 的 RSA 阶段单位是接收者数量，不能复用文件字节数，否则会把密钥封装误判为 100%。
	  this.publish(target, executionId, descriptor, "PROTECTING_KEY", recipients.length, recipients.length, true);
	  await bestEffortProgress(client, task.id, task.current_attempt.id, { stage: "PROTECTING_KEY", processed_bytes: recipients.length, total_bytes: recipients.length }, controller.signal);
      this.publish(target, executionId, descriptor, "UPLOADING", 0, workerResult.ciphertext_size, true);
	  await bestEffortProgress(client, task.id, task.current_attempt.id, { stage: "UPLOADING", processed_bytes: 0, total_bytes: workerResult.ciphertext_size }, controller.signal);
      const uploadStarted = performance.now();
      let lastUploadProgressAt = 0;
      const upload = await uploadWithRecovery(client, task, current.tempPath, workerResult.ciphertext_sha256, controller.signal, (uploadedBytes, totalBytes) => {
        this.publish(target, executionId, descriptor, "UPLOADING", uploadedBytes, totalBytes, true);
        if (Date.now() - lastUploadProgressAt >= 500) {
          lastUploadProgressAt = Date.now();
          progressReports.enqueue(() => bestEffortProgress(client, task.id, task.current_attempt.id, { stage: "UPLOADING", processed_bytes: Math.min(uploadedBytes, totalBytes), total_bytes: totalBytes }, controller.signal));
        }
      });
      // 上传流结束只代表请求体已经发送完；先清空尾部进度请求，避免迟到的 UPLOADING 与下一阶段争抢任务行锁。
      await progressReports.drain();
      this.publish(target, executionId, descriptor, "SAVING_METADATA", 0, 0, false);
	  await bestEffortProgress(client, task.id, task.current_attempt.id, { stage: "SAVING_METADATA", processed_bytes: 0, total_bytes: 0 }, controller.signal);
      const uploadDurationMS = elapsedMillis(uploadStarted);
      const completed = await completeWithRecovery(client, task, descriptor.idempotencyKey, upload, workerResult, handle.size, {
        validationDurationMS,
        uploadDurationMS,
        totalDurationMS: elapsedMillis(totalStarted),
        recipientCount: recipients.length
      }, controller.signal);
	  this.publish(target, executionId, descriptor, "COMPLETED", 0, 0, false);
	  await removeActiveExecution(executionId);
      return { executionId, file: completed };
    } catch (error) {
      const sanitized = createSanitizedExecutionError(error, { executionId, taskId: current.task?.id, stage: controller.signal.aborted ? "CANCELLED" : "FAILED" });
      const task = current.task;
	  if (task) {
		try { await client.fail(task.id, task.current_attempt.id, sanitized.code, sanitized.retryable); await removeActiveExecution(executionId); } catch { /* 网络中断时保留非敏感恢复索引。 */ }
	  }
      this.publish(target, executionId, descriptor, controller.signal.aborted ? "CANCELLED" : "FAILED", 0, handle.size, false);
      throw sanitized;
    } finally {
      worker.terminate();
      if (current.tempPath) await removeTempCiphertext(current.tempPath).catch(() => undefined);
      releaseFileHandle(descriptor.fileHandleId);
      this.progress.finish(executionId);
      this.active.delete(executionId);
    }
  }

  async cancel(executionId: string): Promise<void> {
    const current = this.active.get(executionId);
    if (!current) return;
    current.controller.abort();
    current.worker.terminate();
    if (current.task) {
      const client = new EncryptionTaskApiClient(current.descriptor.apiBaseUrl, current.descriptor.accessToken, current.descriptor.tenantId);
	  try { await client.cancel(current.task.id); await removeActiveExecution(executionId); } catch { /* 下次启动在同一账号租户上下文补报中断。 */ }
    }
  }

  cancelAll(): void {
    for (const current of this.active.values()) { current.controller.abort(); current.worker.terminate(); }
    this.progress.clear();
  }

  private publish(target: WebContents, executionId: string, descriptor: EncryptionExecutionDescriptor, stage: EncryptionProgressEvent["stage"], processedBytes: number, totalBytes: number, cancellable: boolean): void {
    this.progress.publish(target, { executionId, accountId: descriptor.accountId, tenantId: descriptor.tenantId, stage, processedBytes, totalBytes, cancellable });
  }
}

export function normalizedRecipients(descriptor: EncryptionExecutionDescriptor): EncryptionRecipientDescriptor[] {
  if (descriptor.recipients && descriptor.recipients.length > 0) return descriptor.recipients;
  return [{ userId: descriptor.authorization.recipientUserId, rsaPublicKeyId: descriptor.authorization.rsaPublicKeyId, publicKeyPem: descriptor.authorization.publicKeyPem, publicKeyFingerprintSha256: descriptor.authorization.publicKeyFingerprintSha256 }];
}

export function protectedKeyPayloads(workerResult: CryptoWorkerResult): CompletionProtectedKey[] {
  const keys = workerResult.protected_keys && workerResult.protected_keys.length > 0 ? workerResult.protected_keys : [workerResult.protected_key];
  const values = workerResult.protected_keys_base64 && workerResult.protected_keys_base64.length > 0 ? workerResult.protected_keys_base64 : [workerResult.protected_key_base64];
  return keys.map((key, index): CompletionProtectedKey => ({
    recipient_user_id: numericBinding(key.binding, "recipient_user_id"),
    rsa_public_key_id: stringBinding(key.binding, "rsa_public_key_id"),
    algorithm_code: key.algorithm_code,
    algorithm_version: key.algorithm_version,
    format: key.format,
    value_base64: values[index],
    context_sha256: key.context_sha256,
    oaep_hash: "SHA-256",
    oaep_label_sha256: stringBinding(key.binding, "oaep_label_sha256") || key.context_sha256,
    protect_duration_ms: numericOptionalField(key, "protect_duration_ms") ?? numericBinding(key.binding, "protect_duration_ms") ?? 0,
    adapter_binding: key.binding
  }));
}

export interface CompletionTiming {
  validationDurationMS: number;
  uploadDurationMS: number;
  totalDurationMS: number;
  recipientCount: number;
}

export function buildCompletionPayload(upload: CiphertextUploadReceipt, workerResult: CryptoWorkerResult, plaintextSize: number, timing: CompletionTiming): CompleteEncryptionRequest {
  const protectedKeys = protectedKeyPayloads(workerResult);
  const protectedKeyTotalSize = protectedKeys.reduce((total, item) => total + Buffer.byteLength(String(item.value_base64 ?? ""), "base64"), 0);
  const benchmark = {
    validation_duration_ms: timing.validationDurationMS,
    file_encryption_duration_ms: workerResult.aes_encrypt_ms,
    key_protection_duration_ms: workerResult.dek_protect_ms,
    upload_duration_ms: timing.uploadDurationMS,
    metadata_commit_duration_ms: 0,
    total_duration_ms: timing.totalDurationMS,
    recipient_count: timing.recipientCount,
    plaintext_size_bytes: plaintextSize,
    ciphertext_size_bytes: workerResult.ciphertext_size,
    protected_key_total_size_bytes: protectedKeyTotalSize,
    algorithm_code: workerResult.protected_keys?.[0]?.algorithm_code ?? workerResult.protected_key.algorithm_code,
    algorithm_version: workerResult.protected_keys?.[0]?.algorithm_version ?? workerResult.protected_key.algorithm_version,
    client_runtime: "LOCAL_GO_WORKER",
    result: "SUCCESS",
    // 兼容当前后端过渡期字段；统一契约字段稳定后可移除这些别名。
    plaintext_size: plaintextSize,
    ciphertext_size: workerResult.ciphertext_size,
    aes_encrypt_ms: workerResult.aes_encrypt_ms,
    dek_protect_ms: workerResult.dek_protect_ms,
    upload_ms: timing.uploadDurationMS
  };
  return {
    upload_id: upload.upload_id,
    content_encryption: { algorithm: "AES-256-GCM", container_format: "GCPABE01", encryption_version: "1", nonce_prefix_base64: workerResult.nonce_prefix_base64, chunk_size: workerResult.chunk_size, chunk_count: workerResult.chunk_count, tag_length: 16, aad_version: "1", context_sha256: workerResult.context_sha256 },
    protected_keys: protectedKeys,
    benchmark
  };
}

export function sanitizeExecutionError(error: unknown, context: Partial<SanitizedEncryptionError> = {}): SanitizedEncryptionError {
  const rawCode = rawExecutionErrorCode(error);
  const uncertain = isUncertainRemoteError(error);
  const retryable = rawCode !== "WORKER_INTEGRITY_FAILED" && rawCode !== "SAFE_STORAGE_UNAVAILABLE";
  return {
    code: rawCode || "ENCRYPTION_FAILED",
    message: sanitizeExecutionMessage(error, rawCode || "ENCRYPTION_FAILED"),
    retryable,
    stage: context.stage,
    taskId: context.taskId,
    executionId: context.executionId,
    traceId: typeof error === "object" && error && "traceId" in error ? String((error as { traceId?: unknown }).traceId ?? "") : "",
    causeCode: typeof error === "object" && error && "code" in error ? String((error as { code?: unknown }).code ?? "") : rawCode,
    ...(uncertain ? { causeCode: rawCode || "UNKNOWN_REMOTE_ERROR" } : {}),
  } as SanitizedEncryptionError;
}

export function createSanitizedExecutionError(error: unknown, context: Partial<SanitizedEncryptionError> = {}): Error & SanitizedEncryptionError {
  const sanitized = sanitizeExecutionError(error, context);
  return Object.assign(new Error(sanitized.message), sanitized);
}

function rawExecutionErrorCode(error: unknown): string {
  const code = typeof error === "object" && error && "code" in error ? String((error as { code: unknown }).code) : "";
  if (code) return code;
  if (error instanceof Error) return error.message;
  return "ENCRYPTION_FAILED";
}

function sanitizeExecutionMessage(error: unknown, code: string): string {
  if (code === "ENCRYPTION_CANCELLED") return "加密已取消";
  if (typeof error === "object" && error && "message" in error) {
    const message = String((error as { message?: unknown }).message ?? "").trim();
    if (message) return message;
  }
  return `加密任务未完成：${code}`;
}

function isUncertainRemoteError(error: unknown): boolean {
  const status = typeof error === "object" && error && "status" in error ? Number((error as { status?: unknown }).status) : 0;
  const code = rawExecutionErrorCode(error);
  if (status >= 500) return true;
  if (status === 0) return true;
  return ["NETWORK_ERROR", "REMOTE_REQUEST_FAILED", "FETCH_FAILED", "ECONNRESET", "ETIMEDOUT"].includes(code);
}

async function bestEffortProgress(client: EncryptionTaskApiClient, taskId: string, attemptId: string, body: unknown, signal?: AbortSignal): Promise<void> {
  try {
    await client.reportProgress(taskId, attemptId, body, signal);
  } catch {
    return;
  }
}

async function uploadWithRecovery(client: EncryptionTaskApiClient, task: RemoteEncryptionTask, filePath: string, sha256: string, signal: AbortSignal, onProgress?: (uploadedBytes: number, totalBytes: number) => void): Promise<CiphertextUploadReceipt> {
  try {
    return await client.uploadCiphertext(task.id, task.current_attempt.id, filePath, sha256, signal, onProgress);
  } catch (error) {
    if (!isUncertainRemoteError(error)) throw error;
    const remote = await client.task(task.id, signal);
    if (remote.current_attempt.id !== task.current_attempt.id) throw error;
    return await client.uploadCiphertext(task.id, task.current_attempt.id, filePath, sha256, signal, onProgress);
  }
}

async function completeWithRecovery(client: EncryptionTaskApiClient, task: RemoteEncryptionTask, idempotencyKey: string, upload: CiphertextUploadReceipt, workerResult: CryptoWorkerResult, plaintextSize: number, timing: CompletionTiming, signal: AbortSignal): Promise<unknown> {
  const payload = buildCompletionPayload(upload, workerResult, plaintextSize, timing);
  try {
    return await client.complete(task.id, task.current_attempt.id, `${idempotencyKey}:complete`, payload, signal);
  } catch (error) {
    if (!isUncertainRemoteError(error)) throw error;
    const remote = await client.task(task.id, signal);
    if (remote.status === "COMPLETED" || remote.current_attempt.status === "COMPLETED") {
      return { id: remote.file_id, task_id: remote.id, status: remote.status };
    }
    if (remote.current_attempt.id === task.current_attempt.id && remote.current_attempt.status === "SAVING_METADATA") {
      return await client.complete(task.id, task.current_attempt.id, `${idempotencyKey}:complete`, payload, signal);
    }
    throw error;
  }
}

function elapsedMillis(startedAt: number): number {
  return Math.max(0, Math.round(performance.now() - startedAt));
}

function stringBinding(binding: Record<string, unknown>, key: string): string | undefined {
  return typeof binding[key] === "string" ? binding[key] : undefined;
}

function numericBinding(binding: Record<string, unknown>, key: string): number | undefined {
  return typeof binding[key] === "number" && Number.isFinite(binding[key]) ? binding[key] : undefined;
}

function numericOptionalField(value: object, key: string): number | undefined {
  const field = (value as Record<string, unknown>)[key];
  return typeof field === "number" && Number.isFinite(field) ? field : undefined;
}
