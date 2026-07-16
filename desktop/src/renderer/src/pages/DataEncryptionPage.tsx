import { useEffect, useMemo, useState } from "react";
import { Alert, Button, message } from "antd";
import { KeyOutlined, UploadOutlined } from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";
import { API_BASE_URL } from "../api/request";
import { listEncryptionAlgorithms, listMyRSAKeys, listRecipientKeys, listRSARecipients, type AlgorithmCapability, type RSAPublicKey, type RSARecipient } from "../api/encryption";
import { FileSelector, type SelectedEncryptionFile } from "../components/encryption/FileSelector";
import { AlgorithmAuthorizationForm } from "../components/encryption/AlgorithmAuthorizationForm";
import { EncryptionConfirmation, type SelectedRecipientItem } from "../components/encryption/EncryptionConfirmation";
import { EncryptionProgress } from "../components/encryption/EncryptionProgress";
import { RetryAction } from "../components/encryption/RetryAction";
import { PageHeader, PageShell } from "../components/ui";

interface ProgressState {
  executionId: string;
  stage: string;
  processedBytes: number;
  totalBytes: number;
  percent?: number;
  cancellable: boolean;
  startedAt: number;
}

const knownEncryptionErrorCodes = ["ENCRYPTION_CANCELLED", "WORKER_START_FAILED", "WORKER_OPERATION_FAILED", "WORKER_INTEGRITY_FAILED", "ENCRYPTION_CONCURRENCY_LIMITED", "CIPHERTEXT_HASH_MISMATCH", "SAFE_STORAGE_UNAVAILABLE", "FILE_HANDLE_EXPIRED", "FILE_HANDLE_INVALID", "FILE_INVALID", "SOURCE_FILE_CHANGED", "ALGORITHM_UNAVAILABLE", "API_BASE_URL_REJECTED", "TENANT_ID_INVALID", "RSA_PUBLIC_KEY_INVALID", "ENCRYPTION_CONTEXT_INVALID", "AUTH_ACCESS_TOKEN_EXPIRED", "AUTH_ACCESS_TOKEN_INVALID", "TENANT_PERMISSION_DENIED", "RSA_KEY_NOT_FOUND", "ENCRYPTION_ALGORITHM_UNAVAILABLE", "ENCRYPTION_ADMISSION_UNAVAILABLE", "ENCRYPTION_RATE_LIMITED", "ENCRYPTION_FILE_INVALID", "ENCRYPTION_OWNER_KEY_REQUIRED", "PROTECTED_KEY_INVALID", "CIPHERTEXT_UPLOAD_REQUIRED", "ENCRYPTION_STATE_CONFLICT", "CIPHERTEXT_STORAGE_FAILED", "BAD_REQUEST", "INTERNAL_ERROR", "PLATFORM_PERMISSION_DENIED", "PERMISSION_DENIED"];

export function canStartEncryption(input: { file: SelectedEncryptionFile | null; algorithmCode: string; recipientId?: number; selectedKey?: RSAPublicKey; selectedKeys?: RSAPublicKey[]; busy: boolean }): boolean {
  const selectedKeys = input.selectedKeys ?? (input.selectedKey ? [input.selectedKey] : []);
  return Boolean(input.file && input.algorithmCode && selectedKeys.length > 0 && !input.busy);
}

export function selectedRecipientDescriptors(items: Array<{ userId: number; key: RSAPublicKey }>) {
  return items.map((item) => ({ userId: String(item.userId), rsaPublicKeyId: item.key.id, publicKeyPem: item.key.public_key_pem, publicKeyFingerprintSha256: item.key.fingerprint_sha256 }));
}

export function encryptionErrorCode(error: unknown): string {
  const directCode = typeof error === "object" && error && "code" in error ? String((error as { code: unknown }).code) : "";
  if (directCode) return directCode;
  const text = error instanceof Error ? error.message : String(error ?? "");
  return knownEncryptionErrorCodes.find((code) => text.includes(code)) ?? "ENCRYPTION_FAILED";
}

export function DataEncryptionPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const ownerUserId = Number(auth.currentUserId);
  const [algorithms, setAlgorithms] = useState<AlgorithmCapability[]>([]);
  const [recipients, setRecipients] = useState<RSARecipient[]>([]);
  const [myKeys, setMyKeys] = useState<RSAPublicKey[]>([]);
  const [keysByUser, setKeysByUser] = useState<Record<number, RSAPublicKey[]>>({});
  const [algorithmCode, setAlgorithmCode] = useState("");
  const [recipientIds, setRecipientIds] = useState<number[]>([]);
  const [keyIdsByUser, setKeyIdsByUser] = useState<Record<number, string>>({});
  const [file, setFile] = useState<SelectedEncryptionFile | null>(null);
  const [progress, setProgress] = useState<ProgressState | null>(null);
  const [busy, setBusy] = useState(false);
  const [errorCode, setErrorCode] = useState("");
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let active = true;
    setNotice("");
    setRecipientIds([]);
    setKeysByUser({});
    setKeyIdsByUser({});
    void Promise.all([listEncryptionAlgorithms(), listRSARecipients(), listMyRSAKeys()])
      .then(([algorithmItems, recipientItems, keyItems]) => {
        if (!active) return;
        setAlgorithms(algorithmItems);
        setMyKeys(keyItems);
        setAlgorithmCode(algorithmItems[0]?.code ?? "");
        const merged = mergeOwnerRecipient(recipientItems, ownerUserId, auth.user?.nickname || auth.user?.email || "当前用户", keyItems.length);
        setRecipients(merged);
        const owner = merged.find((item) => item.user_id === ownerUserId && item.available && item.active_key_count > 0);
        setRecipientIds(owner ? [owner.user_id] : []);
      })
      .catch((error) => setNotice(error instanceof Error ? error.message : "加载加密配置失败"));
    return () => { active = false; };
  }, [auth.currentUserId, auth.currentTenantId]);

  useEffect(() => {
    if (!window.desktopEncryption || !auth.currentUserId || !auth.currentTenantId || !auth.accessToken) return;
    void window.desktopEncryption.reportInterrupted(auth.currentUserId, auth.currentTenantId, new URL(API_BASE_URL).origin, auth.accessToken)
      .then((count) => { if (count > 0) setNotice(`已将 ${count} 个异常退出的执行标记为可重试失败。`); })
      .catch(() => undefined);
  }, [auth.currentUserId, auth.currentTenantId, auth.accessToken]);

  useEffect(() => {
    let active = true;
    if (recipientIds.length === 0) {
      setKeysByUser({});
      setKeyIdsByUser({});
      return;
    }
    void Promise.all(recipientIds.map(async (userId) => {
      const keys = userId === ownerUserId && myKeys.length > 0 ? myKeys : await listRecipientKeys(userId);
      return [userId, keys] as const;
    })).then((entries) => {
      if (!active) return;
      const nextKeys: Record<number, RSAPublicKey[]> = {};
      const nextKeyIds: Record<number, string> = {};
      for (const [userId, userKeys] of entries) {
        nextKeys[userId] = userKeys;
        nextKeyIds[userId] = keyIdsByUser[userId] && userKeys.some((key) => key.id === keyIdsByUser[userId]) ? keyIdsByUser[userId] : userKeys[0]?.id ?? "";
      }
      setKeysByUser(nextKeys);
      setKeyIdsByUser(nextKeyIds);
    }).catch(() => setNotice("加载接收者公钥失败"));
    return () => { active = false; };
  }, [recipientIds.join(","), myKeys.length, ownerUserId]);

  useEffect(() => window.desktopEncryption?.onProgress((event) => {
    if (event.accountId !== auth.currentUserId || event.tenantId !== auth.currentTenantId) return;
    setProgress((current) => ({
      executionId: event.executionId,
      stage: event.stage,
      processedBytes: event.processedBytes,
      totalBytes: event.totalBytes,
      percent: event.percent,
      cancellable: event.cancellable,
      startedAt: current?.executionId === event.executionId ? current.startedAt : Date.now()
    }));
  }), [auth.currentUserId, auth.currentTenantId]);

  const selectedRecipientItems = useMemo<SelectedRecipientItem[]>(() => recipientIds
    .map((userId) => ({ userId, recipient: recipients.find((item) => item.user_id === userId), key: keysByUser[userId]?.find((item) => item.id === keyIdsByUser[userId]) }))
    .filter((item): item is SelectedRecipientItem => Boolean(item.recipient && item.key)), [recipientIds, recipients, keysByUser, keyIdsByUser]);
  const selectedKeys = selectedRecipientItems.map((item) => item.key);
  const selectedAlgorithm = algorithms.find((item) => item.code === algorithmCode);
  const ready = canStartEncryption({ file, algorithmCode, selectedKeys, busy });
  const runtimeAvailable = Boolean(window.desktopEncryption);

  async function selectFile() {
    if (!window.desktopEncryption) {
      setNotice("请在 Electron 桌面端选择文件。");
      return;
    }
    const selected = await window.desktopEncryption.selectFile();
    setFile(selected);
    setErrorCode("");
    setProgress(null);
  }

  async function start() {
    if (!window.desktopEncryption || !file || selectedRecipientItems.length === 0) return;
    const executionId = crypto.randomUUID();
    const startedAt = Date.now();
    setBusy(true);
    setErrorCode("");
    setNotice("");
    setProgress({ executionId, stage: "VALIDATING", processedBytes: 0, totalBytes: file.size, percent: 0, cancellable: true, startedAt });
    try {
      const descriptors = selectedRecipientDescriptors(selectedRecipientItems);
      const first = descriptors[0];
      await window.desktopEncryption.start(executionId, {
        accountId: auth.currentUserId,
        tenantId: auth.currentTenantId,
        apiBaseUrl: new URL(API_BASE_URL).origin,
        accessToken: auth.accessToken,
        idempotencyKey: crypto.randomUUID(),
        fileHandleId: file.handleId,
        algorithmCode,
        algorithmVersion: selectedAlgorithm?.version ?? "1",
        recipients: descriptors,
        authorization: { type: "RSA_RECIPIENT", recipientUserId: first.userId, rsaPublicKeyId: first.rsaPublicKeyId, publicKeyPem: first.publicKeyPem, publicKeyFingerprintSha256: first.publicKeyFingerprintSha256 }
      });
      setProgress({ executionId, stage: "COMPLETED", processedBytes: 0, totalBytes: 0, percent: 100, cancellable: false, startedAt });
      message.success("文件已完成本地加密并上传。");
      setFile(null);
    } catch (error) {
      setErrorCode(encryptionErrorCode(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <PageShell className="encryption-page">
      <PageHeader title="数据加密" description="选择普通明文文件和目标接收者，Electron 会在本机完成 AES-GCM 加密和 RSA 密钥信封封装后上传密文。" actions={<Button icon={<KeyOutlined />} onClick={() => navigate("/my-rsa-keys")}>我的密钥</Button>} />
      {!runtimeAvailable && <Alert type="warning" showIcon message="浏览器预览仅展示界面，文件加密必须在 Electron 桌面端执行。" />}
      {myKeys.length === 0 && <Alert type="warning" showIcon message="你尚未配置有效 RSA 公钥，无法为自己生成密钥信封。" action={<Button size="small" onClick={() => navigate("/my-rsa-keys")}>前往我的密钥</Button>} />}
      {notice && <Alert type={errorCode ? "error" : "info"} showIcon message={notice} closable onClose={() => setNotice("")} />}
      <FileSelector file={file} disabled={busy} onSelect={() => void selectFile()} onRemove={() => setFile(null)} />
      <AlgorithmAuthorizationForm algorithms={algorithms} algorithmCode={algorithmCode} recipients={recipients} recipientIds={recipientIds} keysByUser={keysByUser} keyIdsByUser={keyIdsByUser} ownerUserId={ownerUserId} disabled={busy} onAlgorithm={setAlgorithmCode} onRecipients={(next) => setRecipientIds(ensureOwnerSelected(next, ownerUserId, recipients))} onKey={(userId, value) => setKeyIdsByUser((current) => ({ ...current, [userId]: value }))} />
      {file && selectedRecipientItems.length > 0 && <EncryptionConfirmation file={file} algorithm={algorithmCode} algorithmVersion={selectedAlgorithm?.version} recipients={selectedRecipientItems} ownerUserId={ownerUserId} />}
      <div className="encryption-submit-bar">
        <Button type="primary" size="large" icon={<UploadOutlined />} disabled={!ready || !runtimeAvailable} loading={busy} onClick={() => void start()}>{ENCRYPTION_SUBMIT_TEXT}</Button>
      </div>
      {progress && <EncryptionProgress {...progress} onCancel={() => void window.desktopEncryption?.cancel(progress.executionId)} />}
      {errorCode && <RetryAction errorCode={errorCode} busy={busy} onRetry={() => { setErrorCode(""); setProgress(null); }} />}
    </PageShell>
  );
}

function mergeOwnerRecipient(recipients: RSARecipient[], ownerUserId: number, ownerName: string, ownerKeyCount: number): RSARecipient[] {
  if (!ownerUserId || recipients.some((item) => item.user_id === ownerUserId)) return recipients;
  return [{ user_id: ownerUserId, display_name: ownerName, available: ownerKeyCount > 0, active_key_count: ownerKeyCount, role: "文件拥有者" }, ...recipients];
}

export const ENCRYPTION_SUBMIT_TEXT = "开始加密并上传";

export function ensureOwnerSelected(next: number[], ownerUserId: number, recipients: RSARecipient[]): number[] {
  const owner = recipients.find((item) => item.user_id === ownerUserId && item.available && item.active_key_count > 0);
  if (!owner) return next;
  return Array.from(new Set([owner.user_id, ...next]));
}
