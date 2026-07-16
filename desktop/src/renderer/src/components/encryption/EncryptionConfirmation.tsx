import { Alert, Card, Descriptions, List, Tag, Typography } from "antd";
import type { RSARecipient, RSAPublicKey } from "../../api/encryption";
import { formatFileSize, shortHash } from "../../utils/fileDisplay";
import type { SelectedEncryptionFile } from "./FileSelector";

export interface SelectedRecipientItem {
  userId: number;
  recipient: RSARecipient;
  key: RSAPublicKey;
}

export function confirmationRecipientSummary(recipients: SelectedRecipientItem[], ownerUserId?: number): { count: number; ownerName: string; names: string[]; keyVersions: number[] } {
  return {
    count: recipients.length,
    ownerName: recipients.find((item) => item.userId === ownerUserId)?.recipient.display_name ?? "—",
    names: recipients.map((item) => item.recipient.display_name),
    keyVersions: recipients.map((item) => item.key.version)
  };
}

export function EncryptionConfirmation({ file, algorithm, algorithmVersion, recipients, ownerUserId }: { file: SelectedEncryptionFile; algorithm: string; algorithmVersion?: string; recipients: SelectedRecipientItem[]; ownerUserId?: number }) {
  const summary = confirmationRecipientSummary(recipients, ownerUserId);
  return (
    <Card title="3. 确认" className="encryption-card">
      <Alert type="info" showIcon message="文件明文、明文数据密钥、私钥和本地文件路径不会上传至服务端，仅上传文件密文和受保护的数据密钥。" />
      <Descriptions column={{ xs: 1, md: 2 }} size="small" style={{ marginTop: 16 }} items={[
        { key: "file", label: "文件", children: <Typography.Text ellipsis={{ tooltip: file.name }}>{file.name}</Typography.Text> },
        { key: "size", label: "文件大小", children: formatFileSize(file.size) },
        { key: "content", label: "文件加密算法", children: "AES-256-GCM" },
        { key: "dek", label: "DEK 保护算法", children: `${algorithm}${algorithmVersion ? ` / v${algorithmVersion}` : ""}` },
        { key: "count", label: "接收者数量", children: `${summary.count} 人` },
        { key: "owner", label: "文件拥有者", children: summary.ownerName }
      ]} />
      <List
        className="confirmation-recipient-list"
        size="small"
        dataSource={recipients}
        renderItem={(item) => (
          <List.Item>
            <Typography.Text>{item.recipient.display_name}</Typography.Text>
            <span>
              {item.userId === ownerUserId && <Tag color="blue">文件拥有者</Tag>}
              <Tag>公钥 v{item.key.version}</Tag>
              <Typography.Text code>{shortHash(item.key.fingerprint_sha256)}</Typography.Text>
            </span>
          </List.Item>
        )}
      />
    </Card>
  );
}
