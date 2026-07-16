import { Avatar, Button, Tooltip, Typography } from "antd";
import { CopyOutlined, UserOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { EncryptedFileRecord, FileOwnerSummary } from "../../api/encryption";
import { fileStatusPresentation, shortHash } from "../../utils/fileDisplay";

export function StatusBadge({ label, tone = "neutral" }: { label: string; tone?: "success" | "processing" | "warning" | "danger" | "neutral" | "info" }) {
  return <span className={`status-badge status-badge-${tone}`}>{label}</span>;
}

export function FileNameCell({ file, localCopy = false }: { file: Pick<EncryptedFileRecord, "original_filename" | "display_mime_type"> & { local_decryption_reveal_token?: string }; localCopy?: boolean }) {
  const extension = file.original_filename.includes(".") ? file.original_filename.split(".").pop()?.toUpperCase() : "FILE";
  return (
    <div className="file-name-cell">
      <Tooltip title={file.original_filename} placement="topLeft"><Typography.Text strong ellipsis>{file.original_filename}</Typography.Text></Tooltip>
      <span>{extension || file.display_mime_type || "文件"} · 加密文件{localCopy || file.local_decryption_reveal_token ? " · 已保存本地副本" : ""}</span>
    </div>
  );
}

export function FingerprintCell({ value, copy = true }: { value?: string; copy?: boolean }) {
  const fingerprint = value || "";
  return (
    <div className="fingerprint-cell">
      <Tooltip title={fingerprint || "暂无"}><Typography.Text code ellipsis>{fingerprint ? shortHash(fingerprint) : "暂无"}</Typography.Text></Tooltip>
      {copy && fingerprint && <Button aria-label="复制指纹" type="text" size="small" icon={<CopyOutlined />} onClick={() => void navigator.clipboard?.writeText(fingerprint)} />}
    </div>
  );
}

export function AlgorithmDisplay({ file, compact = false }: { file: EncryptedFileRecord; compact?: boolean }) {
  const keyAlgorithm = file.algorithm?.dek_algorithm || file.algorithm?.algorithm_code || "未知算法";
  const contentAlgorithm = file.algorithm?.content_algorithm || "AES-256-GCM";
  return (
    <Tooltip title={`${keyAlgorithm} + ${contentAlgorithm}`}>
      <div className={`algorithm-display${compact ? " algorithm-display-compact" : ""}`}>
        <strong>{normalizeAlgorithm(keyAlgorithm)}</strong>
        <span>{normalizeAlgorithm(contentAlgorithm)}</span>
      </div>
    </Tooltip>
  );
}

export function OwnerCell({ owner, ownerUserId }: { owner?: FileOwnerSummary; ownerUserId?: number }) {
  const name = owner?.display_name || owner?.nickname || owner?.email || (ownerUserId ? "未知用户" : "未知");
  return <div className="owner-cell"><Avatar size={28}>{name === "未知" ? <UserOutlined /> : name.slice(0, 1).toUpperCase()}</Avatar><Tooltip title={name}><Typography.Text ellipsis>{name}</Typography.Text></Tooltip></div>;
}

export function ValueWithLabel({ label, children }: { label: string; children: ReactNode }) {
  return <div className="value-with-label"><span>{label}</span><strong>{children}</strong></div>;
}

export function FileStatusBadge({ status }: { status?: string }) {
  const meta = fileStatusPresentation(status);
  const tone = meta.color === "success" ? "success" : meta.color === "processing" ? "processing" : meta.color === "error" ? "danger" : meta.color === "warning" ? "warning" : "neutral";
  return <StatusBadge label={meta.label} tone={tone} />;
}

function normalizeAlgorithm(value: string) {
  if (value.toUpperCase().includes("RSA")) return "RSA-OAEP";
  if (value.toUpperCase().includes("AES")) return "AES-256-GCM";
  return value;
}
