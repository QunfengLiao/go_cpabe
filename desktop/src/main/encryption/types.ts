export type EncryptionStage = "PENDING" | "VALIDATING" | "ENCRYPTING_FILE" | "PROTECTING_KEY" | "UPLOADING" | "SAVING_METADATA" | "DECRYPTING_FILE" | "COMPLETED" | "FAILED" | "CANCELLED";

export interface SelectedFileHandle {
  handleId: string;
  name: string;
  size: number;
  displayMimeType: string;
  lastModifiedMs: number;
}

export interface EncryptionExecutionDescriptor {
  accountId: string;
  tenantId: string;
  apiBaseUrl: string;
  accessToken: string;
  idempotencyKey: string;
  fileHandleId: string;
  algorithmCode: string;
  algorithmVersion: string;
  recipients?: EncryptionRecipientDescriptor[];
  authorization: {
    type: string;
    recipientUserId: string;
    rsaPublicKeyId: string;
    publicKeyPem: string;
    publicKeyFingerprintSha256: string;
  };
}

export interface EncryptionRecipientDescriptor {
  userId: string;
  rsaPublicKeyId: string;
  publicKeyPem: string;
  publicKeyFingerprintSha256: string;
}

export interface EncryptionProgressEvent {
  executionId: string;
  accountId: string;
  tenantId: string;
  stage: EncryptionStage;
  processedBytes: number;
  totalBytes: number;
  uploadedBytes?: number;
  ciphertextBytes?: number;
  protectedRecipients?: number;
  totalRecipients?: number;
  stageElapsedMs?: number;
  totalElapsedMs?: number;
  percent?: number;
  cancellable: boolean;
}

export interface SanitizedEncryptionError {
  code: string;
  message: string;
  retryable: boolean;
  stage?: EncryptionStage;
  taskId?: string;
  executionId?: string;
  traceId?: string;
  causeCode?: string;
}

export interface CryptoWorkerProgress {
  stage: EncryptionStage;
  processed_bytes: number;
  total_bytes: number;
  stage_elapsed_ms?: number;
  total_elapsed_ms?: number;
}

export interface CryptoWorkerResult {
  ciphertext_size: number;
  ciphertext_sha256: string;
  nonce_prefix_base64: string;
  chunk_size: number;
  chunk_count: number;
  context_sha256: string;
  aes_encrypt_ms: number;
  dek_protect_ms: number;
  protected_key_base64: string;
  protected_keys_base64?: string[];
  protected_keys?: Array<{
    algorithm_code: string;
    algorithm_version: string;
    format: string;
    context_sha256: string;
    binding: Record<string, unknown>;
    protect_duration_ms?: number;
  }>;
  protected_key: {
    algorithm_code: string;
    algorithm_version: string;
    format: string;
    context_sha256: string;
    binding: Record<string, unknown>;
  };
}
