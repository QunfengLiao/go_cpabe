/// <reference types="vite/client" />

interface DesktopCredentialStore {
  saveCredential(email: string, password: string): Promise<void>;
  getSavedEmails(): Promise<string[]>;
  getCredentialByEmail(email: string): Promise<string>;
  removeCredential(email: string): Promise<void>;
}

interface DesktopAuthSessionStore {
  getDeviceId(): Promise<string>;
  saveSession(accountId: string, refreshToken: string, expiresAt?: number): Promise<void>;
  hasSession(accountId: string): Promise<boolean>;
  refreshSession(accountId: string): Promise<import("./types").RefreshData>;
  logoutSession(accountId: string): Promise<void>;
  removeSession(accountId: string): Promise<void>;
}

interface Window {
  desktopRuntime?: {
    platform: string;
  };
  desktopCredentialStore?: DesktopCredentialStore;
  desktopAuthSessionStore?: DesktopAuthSessionStore;
  desktopEncryption?: {
    selectFile(): Promise<{ handleId: string; name: string; size: number; displayMimeType: string; lastModifiedMs: number } | null>;
    start(executionId: string, descriptor: {
      accountId: string; tenantId: string; apiBaseUrl: string; accessToken: string; idempotencyKey: string; fileHandleId: string;
      algorithmCode: string; algorithmVersion: string;
      recipients?: Array<{ userId: string; rsaPublicKeyId: string; publicKeyPem: string; publicKeyFingerprintSha256: string }>;
      authorization: { type: string; recipientUserId: string; rsaPublicKeyId: string; publicKeyPem: string; publicKeyFingerprintSha256: string };
    }): Promise<{ executionId: string; file: unknown }>;
    cancel(executionId: string): Promise<void>;
    clearContext(): Promise<void>;
    generateRSAKey(accountId: string, tenantId: string): Promise<{ publicKeyPem: string; fingerprintSha256: string; keyBits: 3072; algorithm: "RSA-OAEP-SHA256" }>;
    importPrivateKey(accountId: string, tenantId: string): Promise<{ publicKeyPem: string; fingerprintSha256: string; keyBits: number; algorithm: "RSA-OAEP-SHA256" } | null>;
    markRSAKeyRegistered(accountId: string, tenantId: string, fingerprint: string, keyId: string, version: number): Promise<void>;
    listPendingRSAKeys(accountId: string, tenantId: string): Promise<Array<{ publicKeyPem: string; fingerprintSha256: string; keyBits: 3072; algorithm: "RSA-OAEP-SHA256" }>>;
    listLocalRSAKeys(accountId: string, tenantId: string): Promise<Array<{ fingerprintSha256: string; publicKeyId?: string; version?: number; registrationPending: boolean; createdAt: string }>>;
    openRSAKeyDirectory(accountId: string, tenantId: string): Promise<boolean>;
    decryptReceivedFile(descriptor: { accountId: string; tenantId: string; apiBaseUrl: string; accessToken: string; fileId: string; suggestedFilename: string }): Promise<{ cancelled: boolean; filename?: string; revealToken?: string; metrics?: {
      ciphertext_download_duration_ms: number;
      key_recovery_duration_ms?: number;
      file_decryption_duration_ms?: number;
      plaintext_write_duration_ms?: number;
      local_decryption_duration_ms?: number;
      total_operation_duration_ms: number;
      ciphertext_size_bytes: number;
      algorithm_code: string;
      algorithm_version: string;
      success: boolean;
      failure_stage?: string;
    } }>;
    decryptFile(descriptor: { accountId: string; tenantId: string; apiBaseUrl: string; accessToken: string; fileId: string; suggestedFilename: string }): Promise<{ cancelled: boolean; outputFilename?: string; filename?: string; decryptionFailed?: boolean; preservedCiphertextFilename?: string; failureCode?: string; revealToken?: string; metrics?: {
      ciphertext_download_duration_ms: number;
      key_recovery_duration_ms?: number;
      file_decryption_duration_ms?: number;
      plaintext_write_duration_ms?: number;
      local_decryption_duration_ms?: number;
      total_operation_duration_ms: number;
      ciphertext_size_bytes: number;
      algorithm_code: string;
      algorithm_version: string;
      success: boolean;
      failure_stage?: string;
    } }>;
    saveCiphertext(descriptor: { accountId: string; tenantId: string; apiBaseUrl: string; accessToken: string; fileId: string; suggestedFilename: string }): Promise<{ cancelled: boolean; outputFilename?: string; revealToken?: string }>;
    revealDecryptedFile(revealToken: string): Promise<boolean>;
    reportInterrupted(accountId: string, tenantId: string, apiBaseUrl: string, accessToken: string): Promise<number>;
    onProgress(listener: (event: { executionId: string; accountId: string; tenantId: string; stage: string; processedBytes: number; totalBytes: number; percent?: number; cancellable: boolean }) => void): () => void;
  };
}
