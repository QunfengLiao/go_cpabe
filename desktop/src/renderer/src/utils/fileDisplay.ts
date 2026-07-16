export type FileStatus =
  | "DRAFT"
  | "PENDING"
  | "VALIDATING"
  | "ENCRYPTING"
  | "ENCRYPTING_FILE"
  | "PROTECTING_KEY"
  | "UPLOADING"
  | "SAVING_METADATA"
  | "AVAILABLE"
  | "FAILED"
  | "CANCELLED"
  | "CLEANUP_PENDING";

export interface RecipientDisplay {
  user_id?: number | string;
  display_name?: string;
  nickname?: string;
  email?: string;
}

export interface EncryptionMetrics {
  validationDurationMs?: number;
  fileEncryptionDurationMs?: number;
  keyProtectionDurationMs?: number;
  localEncryptionDurationMs?: number;
  uploadDurationMs?: number;
  metadataCommitDurationMs?: number;
  totalDurationMs?: number;
  recipientCount?: number;
  protectedKeyTotalSizeBytes?: number;
}

export interface DecryptionMetrics {
  lastSuccessfulAt?: string;
  downloadDurationMs?: number;
  keyRecoveryDurationMs?: number;
  fileDecryptionDurationMs?: number;
  plaintextWriteDurationMs?: number;
  localDecryptionDurationMs?: number;
  totalOperationDurationMs?: number;
  successfulCount?: number;
  averageLocalDecryptionDurationMs?: number;
  minimumLocalDecryptionDurationMs?: number;
  maximumLocalDecryptionDurationMs?: number;
}

const fileStatusMap: Record<string, { label: string; color: string }> = {
  DRAFT: { label: "草稿", color: "default" },
  PENDING: { label: "等待中", color: "processing" },
  VALIDATING: { label: "校验中", color: "processing" },
  ENCRYPTING: { label: "加密中", color: "processing" },
  ENCRYPTING_FILE: { label: "加密中", color: "processing" },
  PROTECTING_KEY: { label: "保护密钥", color: "processing" },
  UPLOADING: { label: "上传中", color: "processing" },
  SAVING_METADATA: { label: "提交元数据", color: "processing" },
  AVAILABLE: { label: "可用", color: "success" },
  FAILED: { label: "失败", color: "error" },
  CANCELLED: { label: "已取消", color: "default" },
  CLEANUP_PENDING: { label: "待清理", color: "warning" }
};

export function formatFileSize(value?: number | null): string {
  if (!Number.isFinite(value ?? Number.NaN) || value == null || value < 0) return "—";
  if (value < 1024) return `${Math.round(value)} B`;
  if (value < 1024 ** 2) return `${trimFixed(value / 1024, 1)} KiB`;
  if (value < 1024 ** 3) return `${trimFixed(value / 1024 ** 2, 1)} MiB`;
  return `${trimFixed(value / 1024 ** 3, 1)} GiB`;
}

export function formatDateTime(value?: string | null): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function formatDuration(value?: number | null): string {
  if (!Number.isFinite(value ?? Number.NaN) || value == null || value < 0) return "—";
  if (value < 1000) return `${Math.round(value)} ms`;
  return `${(value / 1000).toFixed(2)} s`;
}

export function shortHash(value?: string | null): string {
  if (!value) return "—";
  const normalized = value.trim();
  if (normalized.length <= 24) return normalized || "—";
  return `${normalized.slice(0, 12)}…${normalized.slice(-8)}`;
}

export function fileStatusPresentation(status?: string | null) {
  return fileStatusMap[String(status ?? "").toUpperCase()] ?? { label: "未知", color: "default" };
}

export function summarizeRecipients(recipients?: RecipientDisplay[] | null, recipientCount?: number | null): string {
  const names = (recipients ?? []).map(recipientName).filter(Boolean);
  if (names.length === 0) return recipientCount && recipientCount > 0 ? `${recipientCount} 人` : "—";
  if (names.length === 1) return names[0];
  if (names.length === 2) return `${names[0]}、${names[1]}`;
  return `${names[0]}、${names[1]}等 ${recipientCount && recipientCount > names.length ? recipientCount : names.length} 人`;
}

export function recipientName(recipient?: RecipientDisplay | null): string {
  return recipient?.display_name || recipient?.nickname || recipient?.email || "";
}

export function localEncryptionDuration(record: Record<string, unknown>): string {
  return formatDuration(toEncryptionMetrics(record).localEncryptionDurationMs);
}

export function localDecryptionDuration(record: Record<string, unknown>): string {
  const duration = toDecryptionMetrics(record).localDecryptionDurationMs;
  return duration == null ? "未解密" : formatDuration(duration);
}

export function toEncryptionMetrics(record: Record<string, unknown>): EncryptionMetrics {
  const benchmark = objectValue(record.benchmark) ?? objectValue(record.encryption_metrics) ?? objectValue(record.encryptionMetrics) ?? {};
  const fileEncryptionDurationMs = numberValue(record.aes_encrypt_ms ?? benchmark.aes_encrypt_ms ?? benchmark.file_encryption_duration_ms ?? benchmark.fileEncryptionDurationMs);
  const keyProtectionDurationMs = numberValue(record.dek_protect_ms ?? benchmark.dek_protect_ms ?? benchmark.key_protection_duration_ms ?? benchmark.keyProtectionDurationMs);
  const localEncryptionDurationMs = numberValue(benchmark.local_encryption_duration_ms ?? benchmark.localEncryptionDurationMs) ?? sumDefined(fileEncryptionDurationMs, keyProtectionDurationMs);
  return {
    validationDurationMs: numberValue(benchmark.validation_duration_ms ?? benchmark.validationDurationMs),
    fileEncryptionDurationMs,
    keyProtectionDurationMs,
    localEncryptionDurationMs,
    uploadDurationMs: numberValue(benchmark.upload_ms ?? benchmark.upload_duration_ms ?? benchmark.uploadDurationMs),
    metadataCommitDurationMs: numberValue(benchmark.metadata_commit_ms ?? benchmark.metadataCommitDurationMs),
    totalDurationMs: numberValue(benchmark.total_ms ?? benchmark.totalDurationMs),
    recipientCount: numberValue(record.recipient_count ?? benchmark.recipient_count ?? benchmark.recipientCount),
    protectedKeyTotalSizeBytes: numberValue(benchmark.protected_key_total_size ?? benchmark.protected_key_total_size_bytes ?? benchmark.protectedKeyTotalSizeBytes)
  };
}

export function toDecryptionMetrics(record: Record<string, unknown>): DecryptionMetrics {
  const metrics = objectValue(record.decryption_metrics) ?? objectValue(record.decryptionMetrics) ?? {};
  return {
    lastSuccessfulAt: stringValue(metrics.last_successful_at ?? metrics.lastSuccessfulAt),
    downloadDurationMs: numberValue(metrics.ciphertext_download_duration_ms ?? metrics.download_duration_ms ?? metrics.downloadDurationMs),
    keyRecoveryDurationMs: numberValue(metrics.key_recovery_duration_ms ?? metrics.keyRecoveryDurationMs),
    fileDecryptionDurationMs: numberValue(metrics.file_decryption_duration_ms ?? metrics.fileDecryptionDurationMs),
    plaintextWriteDurationMs: numberValue(metrics.plaintext_write_duration_ms ?? metrics.plaintextWriteDurationMs),
    localDecryptionDurationMs: numberValue(metrics.local_decryption_duration_ms ?? metrics.localDecryptionDurationMs),
    totalOperationDurationMs: numberValue(metrics.total_operation_duration_ms ?? metrics.totalOperationDurationMs),
    successfulCount: numberValue(metrics.successful_count ?? metrics.successfulCount),
    averageLocalDecryptionDurationMs: numberValue(metrics.average_local_decryption_duration_ms ?? metrics.averageLocalDecryptionDurationMs),
    minimumLocalDecryptionDurationMs: numberValue(metrics.minimum_local_decryption_duration_ms ?? metrics.minimumLocalDecryptionDurationMs),
    maximumLocalDecryptionDurationMs: numberValue(metrics.maximum_local_decryption_duration_ms ?? metrics.maximumLocalDecryptionDurationMs)
  };
}

function trimFixed(value: number, digits: number): string {
  return value.toFixed(digits);
}

function numberValue(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value ? value : undefined;
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

function sumDefined(left?: number, right?: number): number | undefined {
  if (left == null && right == null) return undefined;
  return (left ?? 0) + (right ?? 0);
}
