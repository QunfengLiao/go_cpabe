import { getAuthRuntime } from "./authRuntime";
import { API_BASE_URL, ApiError, clearRequestCache, request } from "./request";

export interface AlgorithmCapability {
  code: string;
  display_name: string;
  category: string;
  version: string;
  authorization_type: string;
  enabled?: boolean;
}

export interface RSAPublicKey {
  id: string;
  user_id: number;
  version: number;
  fingerprint_sha256: string;
  public_key_pem: string;
  key_bits: number;
  algorithm: string;
  status: string;
  created_at: string;
}

export interface RSARecipient {
  user_id: number;
  display_name: string;
  email?: string;
  role?: string;
  available: boolean;
  active_key_count: number;
}

export interface FileRecipientSummary {
  user_id?: number;
  display_name?: string;
  nickname?: string;
  email?: string;
  public_key_version?: number;
  public_key_fingerprint_sha256?: string;
  protect_duration_ms?: number;
}

export interface FileOwnerSummary {
  user_id?: number;
  display_name?: string;
  nickname?: string;
  email?: string;
}

export interface FileAlgorithmSummary {
  content_algorithm?: string;
  dek_algorithm?: string;
  algorithm_code?: string;
  algorithm_version?: string;
  metadata_version?: string;
}

export interface FileBenchmarkSummary {
  aes_encrypt_ms?: number;
  dek_protect_ms?: number;
  average_recipient_protect_ms?: number;
  min_recipient_protect_ms?: number;
  max_recipient_protect_ms?: number;
  upload_ms?: number;
  metadata_commit_ms?: number;
  total_ms?: number;
  recipient_count?: number;
  plaintext_size?: number;
  ciphertext_size?: number;
  protected_key_total_size?: number;
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
  ciphertext_download_duration_ms?: number;
  key_recovery_duration_ms?: number;
  file_decryption_duration_ms?: number;
  plaintext_write_duration_ms?: number;
  local_decryption_duration_ms?: number;
  total_operation_duration_ms?: number;
  successful_count?: number;
  average_local_decryption_duration_ms?: number;
  minimum_local_decryption_duration_ms?: number;
  maximum_local_decryption_duration_ms?: number;
}

export interface EncryptedFileRecord {
  id: string;
  original_filename: string;
  display_mime_type?: string;
  plaintext_size: number;
  ciphertext_size?: number;
  ciphertext_sha256?: string;
  owner_user_id?: number;
  owner?: FileOwnerSummary;
  status: "DRAFT" | "PENDING" | "VALIDATING" | "ENCRYPTING" | "UPLOADING" | "AVAILABLE" | "FAILED" | "CANCELLED" | "CLEANUP_PENDING";
  recipients?: FileRecipientSummary[];
  recipient_count?: number;
  algorithm?: FileAlgorithmSummary;
  aes_encrypt_ms?: number;
  dek_protect_ms?: number;
  benchmark?: FileBenchmarkSummary;
  encryption_metrics?: EncryptionMetrics;
  decryption_metrics?: DecryptionMetrics;
  local_decryption_reveal_token?: string;
  created_at: string;
  completed_at?: string;
}

export interface EncryptedFilePage {
  items: EncryptedFileRecord[];
  total: number;
  page: number;
  page_size: number;
}

export type ReceivedFilePage = EncryptedFilePage;
export type FileCenterScope = "tenant_cloud" | "owned_by_me";

export interface FileCenterDetail {
  file: EncryptedFileRecord;
  owner?: FileOwnerSummary;
  recipients: FileRecipientSummary[];
  benchmark?: FileBenchmarkSummary;
  ciphertext?: { ciphertext_sha256?: string; ciphertext_size?: number; status?: string; content_algorithm?: string; encryption_version?: string; nonce_prefix_base64?: string; authentication_tag_length?: number; aad_version?: string };
}

export interface OwnDecryptionMaterial {
  file_id: string;
  original_filename: string;
  plaintext_size: number;
  protected_key_base64: string;
  context_sha256: string;
  rsa_public_key_id: string;
  public_key_fingerprint_sha256: string;
  key_envelopes?: Array<{
    key_id: string;
    protected_key_base64: string;
    context_sha256: string;
    algorithm_code: string;
    algorithm_version: string;
    protected_key_format: string;
    rsa_public_key_id: string;
    public_key_fingerprint_sha256: string;
    oaep_hash?: string;
  }>;
}

export function listEncryptionAlgorithms(): Promise<AlgorithmCapability[]> {
  return request("/tenant/encryption-algorithms");
}

export function listRSARecipients(): Promise<RSARecipient[]> {
  return request("/tenant/rsa-recipients");
}

export function listRecipientKeys(userId: number): Promise<RSAPublicKey[]> {
  return request(`/tenant/rsa-recipients/${userId}/public-keys`);
}

export function listMyRSAKeys(): Promise<RSAPublicKey[]> {
  return request("/tenant/me/rsa-public-keys");
}

export function registerMyRSAKey(input: { publicKeyPem: string; fingerprintSha256: string }): Promise<RSAPublicKey> {
  return request("/tenant/me/rsa-public-keys", {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify({
      public_key_pem: input.publicKeyPem,
      fingerprint_sha256: input.fingerprintSha256,
      key_bits: 3072,
      algorithm: "RSA-OAEP-SHA256"
    })
  });
}

export function listEncryptedFiles(page = 1, pageSize = 20, status = ""): Promise<EncryptedFilePage> {
  const filter = status ? `&status=${encodeURIComponent(status)}` : "";
  return request(`/tenant/encrypted-files?page=${page}&page_size=${pageSize}${filter}`);
}

export function listReceivedFiles(page = 1, pageSize = 20): Promise<ReceivedFilePage> {
  return request(`/tenant/received-files?page=${page}&page_size=${pageSize}`);
}

export function listFileCenterItems(scope: FileCenterScope, page = 1, pageSize = 20): Promise<EncryptedFilePage> {
  return request(`/tenant/files?scope=${encodeURIComponent(scope)}&page=${page}&page_size=${pageSize}`);
}

export function getFileCenterDetail(fileId: string): Promise<FileCenterDetail> {
  return request(`/tenant/files/${encodeURIComponent(fileId)}`);
}

export async function downloadFileCiphertext(file: EncryptedFileRecord): Promise<void> {
  return downloadCiphertextFromPath(`/tenant/files/${encodeURIComponent(file.id)}/ciphertext`, file);
}

export function getOwnDecryptionMaterial(fileId: string): Promise<OwnDecryptionMaterial> {
  return request(`/tenant/files/${encodeURIComponent(fileId)}/decryption-material`);
}

export function getEncryptedFile(fileId: string): Promise<Record<string, unknown>> {
  return request(`/tenant/encrypted-files/${fileId}`);
}

export function retryEncryptionTask(taskId: string): Promise<unknown> {
  return request(`/tenant/encryption-tasks/${taskId}/retry`, { method: "POST", headers: { "Idempotency-Key": crypto.randomUUID() } });
}

export async function downloadCiphertext(file: EncryptedFileRecord): Promise<void> {
  return downloadCiphertextFromPath(`/tenant/encrypted-files/${file.id}/ciphertext`, file);
}

export async function downloadReceivedCiphertext(file: EncryptedFileRecord): Promise<void> {
  return downloadCiphertextFromPath(`/tenant/received-files/${file.id}/ciphertext`, file);
}

async function downloadCiphertextFromPath(path: string, file: EncryptedFileRecord): Promise<void> {
  const auth = getAuthRuntime();
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: { Authorization: `Bearer ${auth.accessToken}`, "X-Tenant-Id": auth.currentTenantId }
  });
  if (!response.ok) throw new ApiError("密文下载失败", response.status);
  const expected = response.headers.get("X-Ciphertext-SHA256");
  if (!expected || !/^[0-9a-f]{64}$/i.test(expected)) throw new ApiError("下载响应缺少有效摘要", 502, "CIPHERTEXT_DIGEST_MISSING");
  const blob = await response.blob();
  const actual = [...new Uint8Array(await crypto.subtle.digest("SHA-256", await blob.arrayBuffer()))]
    .map((value) => value.toString(16).padStart(2, "0"))
    .join("");
  if (actual !== expected.toLowerCase()) throw new ApiError("下载密文摘要不一致", 422, "CIPHERTEXT_HASH_MISMATCH");
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `${file.original_filename}.enc`;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function clearEncryptionCaches(): void {
  clearRequestCache();
}
