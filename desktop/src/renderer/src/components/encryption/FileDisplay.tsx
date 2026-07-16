import { Avatar, Space, Typography } from "antd";
import { UserOutlined } from "@ant-design/icons";
import type { EncryptedFileRecord, FileOwnerSummary, FileRecipientSummary } from "../../api/encryption";
import { AlgorithmDisplay, FileNameCell, FileStatusBadge, OwnerCell } from "../ui/DataDisplay";
import {
  formatDateTime,
  formatDuration,
  formatFileSize,
  localDecryptionDuration,
  localEncryptionDuration,
  recipientName,
  shortHash,
  summarizeRecipients,
  toDecryptionMetrics,
  toEncryptionMetrics
} from "../../utils/fileDisplay";

export function FileStatusTag({ status }: { status?: string }) {
  return <FileStatusBadge status={status} />;
}

export function FileIdentity({ file }: { file: Pick<EncryptedFileRecord, "original_filename" | "display_mime_type"> }) {
  return <FileNameCell file={file} />;
}

export function OwnerText({ owner, ownerUserId }: { owner?: FileOwnerSummary; ownerUserId?: number }) {
  return <OwnerCell owner={owner} ownerUserId={ownerUserId} />;
}

export function RecipientSummary({ recipients, count }: { recipients?: FileRecipientSummary[]; count?: number }) {
  return <Typography.Text>{summarizeRecipients(recipients, count)}</Typography.Text>;
}

export function RecipientAvatarGroup({ recipients, count }: { recipients?: FileRecipientSummary[]; count?: number }) {
  const named = (recipients ?? []).filter((recipient) => recipientName(recipient));
  if (named.length === 0) return <Typography.Text type="secondary">{count ? `${count} 人` : "未知"}</Typography.Text>;
  return (
    <Space size={6}>
      <Avatar.Group max={{ count: 3 }} size="small">
        {named.map((recipient, index) => <Avatar key={`${recipient.user_id ?? index}`} icon={<UserOutlined />}>{recipientName(recipient).slice(0, 1)}</Avatar>)}
      </Avatar.Group>
      <Typography.Text>{summarizeRecipients(named, count)}</Typography.Text>
    </Space>
  );
}

export function AlgorithmText({ file }: { file: EncryptedFileRecord }) {
  return <AlgorithmDisplay file={file} compact />;
}

export function LocalEncryptionDuration({ file }: { file: EncryptedFileRecord }) {
  return <Typography.Text>{localEncryptionDuration(file as unknown as Record<string, unknown>)}</Typography.Text>;
}

export function LocalDecryptionDuration({ file }: { file: EncryptedFileRecord }) {
  return <Typography.Text>{localDecryptionDuration(file as unknown as Record<string, unknown>)}</Typography.Text>;
}

export function FileMetricGrid({ file, detail }: { file?: Partial<EncryptedFileRecord>; detail?: Record<string, unknown> | null }) {
  const task = (detail?.task ?? {}) as Record<string, unknown>;
  const ciphertext = (detail?.ciphertext ?? {}) as Record<string, unknown>;
  const benchmark = { ...(file ?? {}), task, benchmark: file?.benchmark } as Record<string, unknown>;
  const metrics = toEncryptionMetrics(benchmark);
  const ciphertextSize = numberValue(file?.ciphertext_size ?? benchmark.ciphertext_size ?? ciphertext.ciphertext_size);
  return (
    <div className="metric-grid">
      <Metric label="文件校验耗时" value={formatDuration(metrics.validationDurationMs)} />
      <Metric label="AES 文件加密" value={formatDuration(metrics.fileEncryptionDurationMs)} />
      <Metric label="DEK 保护耗时" value={formatDuration(metrics.keyProtectionDurationMs)} />
      <Metric label="本地加密总耗时" value={formatDuration(metrics.localEncryptionDurationMs)} />
      <Metric label="上传耗时" value={formatDuration(metrics.uploadDurationMs)} />
      <Metric label="元数据提交" value={formatDuration(metrics.metadataCommitDurationMs)} />
      <Metric label="任务总耗时" value={formatDuration(metrics.totalDurationMs)} />
      <Metric label="接收者数量" value={metrics.recipientCount == null ? "未知" : `${metrics.recipientCount} 人`} />
      <Metric label="密文大小" value={formatFileSize(ciphertextSize)} />
      <Metric label="受保护 DEK 总大小" value={formatFileSize(metrics.protectedKeyTotalSizeBytes)} />
    </div>
  );
}

export function DecryptionMetricGrid({ file }: { file?: Partial<EncryptedFileRecord> }) {
  const metrics = toDecryptionMetrics((file ?? {}) as Record<string, unknown>);
  return (
    <div className="metric-grid">
      <Metric label="最近一次密钥恢复" value={formatDuration(metrics.keyRecoveryDurationMs)} />
      <Metric label="最近一次 AES 解密" value={formatDuration(metrics.fileDecryptionDurationMs)} />
      <Metric label="最近一次本地解密" value={formatDuration(metrics.localDecryptionDurationMs)} />
      <Metric label="下载耗时" value={formatDuration(metrics.downloadDurationMs)} />
      <Metric label="明文写入耗时" value={formatDuration(metrics.plaintextWriteDurationMs)} />
      <Metric label="操作总耗时" value={formatDuration(metrics.totalOperationDurationMs)} />
      <Metric label="成功解密次数" value={metrics.successfulCount == null ? "未知" : `${metrics.successfulCount} 次`} />
      <Metric label="平均解密耗时" value={formatDuration(metrics.averageLocalDecryptionDurationMs)} />
    </div>
  );
}

export function IntegrityLine({ hash }: { hash?: string }) {
  return <Typography.Text code className="hash-inline">{shortHash(hash)}</Typography.Text>;
}

export function detailDate(value?: unknown) {
  return formatDateTime(typeof value === "string" ? value : undefined);
}

export function algorithmSummary(file: EncryptedFileRecord): string {
  const raw = file.algorithm?.dek_algorithm || file.algorithm?.algorithm_code || "";
  if (!raw) return "未知算法";
  return `${raw} / AES-256-GCM`;
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function numberValue(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}
