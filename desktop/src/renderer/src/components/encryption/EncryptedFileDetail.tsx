import { Button, Descriptions, Empty, List, Space, Tooltip, Typography, message } from "antd";
import { CopyOutlined, DownloadOutlined, FolderOpenOutlined, UnlockOutlined } from "@ant-design/icons";
import type { EncryptedFileRecord, FileRecipientSummary } from "../../api/encryption";
import { formatDateTime, formatFileSize, recipientName, shortHash } from "../../utils/fileDisplay";
import { DetailDrawer } from "../ui";
import { FileIdentity, FileMetricGrid, FileStatusTag, IntegrityLine } from "./FileDisplay";
import { decryptActionText } from "./FileTable";

interface EncryptedFileDetailProps {
  open: boolean;
  detail: Record<string, unknown> | null;
  fallbackFile?: EncryptedFileRecord | null;
  decrypting?: boolean;
  downloading?: boolean;
  retrying?: boolean;
  onClose: () => void;
  onDecrypt?: (file: EncryptedFileRecord) => void;
  onDownload?: (file: EncryptedFileRecord) => void;
  onRetry?: (taskId: string) => void;
}

export function EncryptedFileDetail({ open, detail, fallbackFile, decrypting = false, downloading = false, retrying = false, onClose, onDecrypt, onDownload, onRetry }: EncryptedFileDetailProps) {
  const file = { ...(fallbackFile ?? {}), ...((detail?.file ?? {}) as Record<string, unknown>) } as EncryptedFileRecord;
  const task = (detail?.task ?? {}) as Record<string, unknown>;
  const ciphertext = (detail?.ciphertext ?? {}) as Record<string, unknown>;
  const protectedKey = (detail?.protected_key ?? {}) as Record<string, unknown>;
  const authorization = (detail?.authorization ?? {}) as Record<string, unknown>;
  const recipients = resolveRecipients(file, authorization, detail?.recipients);
  const canRetry = String(task.status ?? "") === "FAILED" && Boolean(task.retryable);
  const digest = stringValue(file.ciphertext_sha256 ?? ciphertext.ciphertext_sha256);
  const hasRevealToken = Boolean(fallbackFile?.local_decryption_reveal_token);
  const decryptButtonText = decryptActionText(fallbackFile ?? file);

  return (
    <DetailDrawer
      className="encrypted-file-detail-drawer"
      title="加密文件详情"
      open={open}
      onClose={onClose}
      width={640}
      extra={<Button onClick={onClose}>关闭</Button>}
      footer={
        <div className="drawer-actions">
          {fallbackFile && onDecrypt && <Button icon={hasRevealToken ? <FolderOpenOutlined /> : <UnlockOutlined />} type="primary" loading={decrypting} disabled={file.status !== "AVAILABLE" && !hasRevealToken} onClick={() => onDecrypt(fallbackFile)}>{decryptButtonText}</Button>}
          {fallbackFile && onDownload && <Button icon={<DownloadOutlined />} loading={downloading} onClick={() => onDownload(fallbackFile)}>下载密文</Button>}
          {canRetry && <Button type="primary" loading={retrying} disabled={retrying} onClick={() => onRetry?.(String(task.id))}>创建新执行重试</Button>}
        </div>
      }
    >
      <div className="detail-stack">
        <section className="detail-section">
          <div className="detail-section-title">文件基本信息</div>
          <FileIdentity file={{ original_filename: file.original_filename ?? "—", display_mime_type: file.display_mime_type }} />
          <Descriptions column={1} size="small" items={[
            { key: "status", label: "状态", children: <FileStatusTag status={file.status} /> },
            { key: "owner", label: "文件拥有者", children: ownerText(file) },
            { key: "plain", label: "原始大小", children: formatFileSize(file.plaintext_size) },
            { key: "cipher", label: "密文大小", children: formatFileSize(numberValue(file.ciphertext_size ?? ciphertext.ciphertext_size)) },
            { key: "created", label: "创建时间", children: formatDateTime(file.created_at) },
            { key: "completed", label: "完成时间", children: formatDateTime(file.completed_at) }
          ]} />
        </section>

        <section className="detail-section">
          <div className="detail-section-title">加密信息</div>
          <Descriptions column={1} size="small" items={[
            { key: "content", label: "文件内容加密", children: file.algorithm?.content_algorithm || "AES-256-GCM" },
            { key: "dek", label: "DEK 保护算法", children: file.algorithm?.dek_algorithm || stringValue(protectedKey.algorithm_code ?? task.algorithm_code) },
            { key: "version", label: "算法版本", children: file.algorithm?.algorithm_version || stringValue(protectedKey.algorithm_version ?? task.algorithm_version) },
            { key: "recipient-count", label: "接收者数量", children: file.recipient_count ?? (recipients.length || "—") },
            { key: "metadata", label: "元数据版本", children: file.algorithm?.metadata_version || "—" }
          ]} />
        </section>

        <section className="detail-section">
          <div className="detail-section-title">密钥信封接收者</div>
          {recipients.length > 0 ? (
            <List
              size="small"
              dataSource={recipients}
              renderItem={(recipient) => (
                <List.Item>
                  <div className="recipient-detail-row">
                    <div>
                      <strong>{recipientName(recipient) || "未知用户"}</strong>
                      <span>{recipient.email || "账号信息缺失"}</span>
                    </div>
                    <div>
                      <span>公钥 v{recipient.public_key_version ?? "—"}</span>
                      <Typography.Text code>{shortHash(recipient.public_key_fingerprint_sha256)}</Typography.Text>
                    </div>
                  </div>
                </List.Item>
              )}
            />
          ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无接收者信息" />}
        </section>

        {(
          <section className="detail-section">
            <div className="detail-section-title">加密性能</div>
            <FileMetricGrid file={file} detail={detail} />
          </section>
        )}

        <section className="detail-section">
          <div className="detail-section-title">完整性信息</div>
          <Descriptions column={1} size="small" items={[
            { key: "sha", label: "密文 SHA-256", children: <Space><IntegrityLine hash={digest} />{digest && <Tooltip title="复制完整哈希"><Button icon={<CopyOutlined />} size="small" onClick={() => void copyIntegrityHash(digest)} /></Tooltip>}</Space> },
            { key: "size", label: "密文大小", children: formatFileSize(numberValue(file.ciphertext_size ?? ciphertext.ciphertext_size)) },
            { key: "storage", label: "存储状态", children: stringValue(ciphertext.status) }
          ]} />
        </section>
      </div>
    </DetailDrawer>
  );
}

export const ENCRYPTED_FILE_DETAIL_SECTIONS = ["文件基本信息", "加密信息", "密钥信封接收者", "性能指标", "完整性信息"] as const;

export function resolveRecipients(file: EncryptedFileRecord, authorization: Record<string, unknown>, detailRecipients?: unknown): FileRecipientSummary[] {
  if (file.recipients?.length) return file.recipients;
  if (Array.isArray(detailRecipients)) {
    return detailRecipients.map((item) => {
      const recipient = item as Record<string, unknown>;
      const user = (recipient.user ?? {}) as Record<string, unknown>;
      return {
        user_id: numberValue(user.id ?? recipient.user_id),
        display_name: stringValue(user.display_name ?? recipient.display_name),
        email: stringValue(user.email ?? recipient.email),
        public_key_version: numberValue(recipient.public_key_version),
        public_key_fingerprint_sha256: stringValue(recipient.fingerprint_sha256 ?? recipient.public_key_fingerprint_sha256),
        protect_duration_ms: numberValue(recipient.protect_duration_ms)
      };
    });
  }
  const recipientId = numberValue(authorization.recipient_user_id);
  const fingerprint = stringValue(authorization.public_key_fingerprint_sha256);
  if (!recipientId && !fingerprint) return [];
  return [{ user_id: recipientId, public_key_fingerprint_sha256: fingerprint }];
}

function ownerText(file: EncryptedFileRecord) {
  return file.owner?.display_name || file.owner?.nickname || file.owner?.email || (file.owner_user_id ? "未知用户" : "—");
}

function stringValue(value: unknown): string {
  return typeof value === "string" && value ? value : "—";
}

function numberValue(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export async function copyIntegrityHash(value: string, clipboard: Pick<Clipboard, "writeText"> | undefined = navigator.clipboard, notify: (content: string) => void = (content) => { message.success(content); }): Promise<void> {
  await clipboard?.writeText(value);
  notify("已复制完整哈希");
}
