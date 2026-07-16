import { Button, type MenuProps, type TableProps } from "antd";
import { CopyOutlined, DownloadOutlined, EyeOutlined, FolderOpenOutlined } from "@ant-design/icons";
import type { EncryptedFileRecord, FileCenterScope } from "../../api/encryption";
import { DataTable, EmptyState, FileNameCell, OwnerCell, AlgorithmDisplay, RowActions } from "../ui";
import { formatDateTime, formatFileSize } from "../../utils/fileDisplay";

interface FileTableProps {
  scope: FileCenterScope;
  items: EncryptedFileRecord[];
  total: number;
  page: number;
  loading?: boolean;
  workingId?: string;
  onPage: (page: number) => void;
  onDecrypt: (file: EncryptedFileRecord) => void;
  onOpenFolder?: (file: EncryptedFileRecord) => void;
  onDownload: (file: EncryptedFileRecord) => void;
  onDetail: (file: EncryptedFileRecord) => void;
  emptySearch?: boolean;
}

/** FileTable 只呈现密文仓库记录；解密能力不参与列表筛选，也不决定下载按钮是否可用。 */
export function FileTable({ scope, items, total, page, loading, workingId, onPage, onDecrypt, onOpenFolder, onDownload, onDetail, emptySearch = false }: FileTableProps) {
  const columns = fileTableColumns({ scope, workingId, onDecrypt, onOpenFolder, onDownload, onDetail });
  return (
    <DataTable
      rowKey="id"
      className="file-center-table"
      loading={loading}
      dataSource={items}
      columns={columns}
      locale={{ emptyText: <EmptyState description={emptyTextForScope(scope)} search={emptySearch} /> }}
      pagination={total > 0 ? { current: page, pageSize: 20, total, onChange: onPage, showSizeChanger: false, showTotal: (value) => `共 ${value} 个密文` } : false}
      size="middle"
    />
  );
}

export function fileTableColumnTitles(scope: FileCenterScope): string[] {
  return tableColumnSchema(scope).map((item) => item.title);
}

export function decryptActionText(file: Pick<EncryptedFileRecord, "local_decryption_reveal_token">): string {
  return file.local_decryption_reveal_token ? "打开" : "下载";
}

function fileTableColumns({ scope, workingId, onDecrypt, onOpenFolder, onDownload, onDetail }: Pick<FileTableProps, "scope" | "workingId" | "onDecrypt" | "onOpenFolder" | "onDownload" | "onDetail">): TableProps<EncryptedFileRecord>["columns"] {
  return tableColumnSchema(scope).map((schema) => {
    if (schema.key === "file") return { title: schema.title, dataIndex: "original_filename", className: "file-column", render: (_value: unknown, file: EncryptedFileRecord) => <button className="file-link-cell" type="button" onClick={() => onDetail(file)}><FileNameCell file={file} localCopy={Boolean(file.local_decryption_reveal_token)} /></button> };
    if (schema.key === "owner") return { title: schema.title, dataIndex: "owner", responsive: ["md"], render: (_value: unknown, file: EncryptedFileRecord) => <OwnerCell owner={file.owner} ownerUserId={file.owner_user_id} /> };
    if (schema.key === "algorithm") return { title: schema.title, dataIndex: "algorithm", responsive: ["lg"], render: (_value: unknown, file: EncryptedFileRecord) => <AlgorithmDisplay file={file} compact /> };
    if (schema.key === "ciphertextSize") return { title: schema.title, dataIndex: "ciphertext_size", responsive: ["md"], render: (value: number | undefined) => value == null ? <TypographyUnknown /> : formatFileSize(value) };
    if (schema.key === "createdAt") return { title: schema.title, dataIndex: "created_at", responsive: ["lg"], render: (value: string) => <span className="nowrap-cell">{formatDateTime(value)}</span> };
    return { title: schema.title, key: "actions", className: "file-actions-column", render: (_value: unknown, file: EncryptedFileRecord) => <FileActions workingId={workingId} file={file} onDecrypt={onDecrypt} onOpenFolder={onOpenFolder} onDownload={onDownload} onDetail={onDetail} /> };
  });
}

function FileActions({ workingId, file, onDecrypt, onOpenFolder, onDownload, onDetail }: Pick<FileTableProps, "workingId" | "onDecrypt" | "onOpenFolder" | "onDownload" | "onDetail"> & { file: EncryptedFileRecord }) {
  const hasRevealToken = Boolean(file.local_decryption_reveal_token);
  const canDownload = file.status === "AVAILABLE" || hasRevealToken;
  const menuItems: MenuProps["items"] = [
    { key: "detail", icon: <EyeOutlined />, label: "查看详情", onClick: () => onDetail(file) },
    { key: "ciphertext", icon: <DownloadOutlined />, label: "仅保存密文", disabled: file.status !== "AVAILABLE", onClick: () => onDownload(file) },
    ...(hasRevealToken ? [{ key: "folder", icon: <FolderOpenOutlined />, label: "打开所在文件夹", onClick: () => (onOpenFolder ?? onDecrypt)(file) }] : []),
    { type: "divider" },
    { key: "copy-id", icon: <CopyOutlined />, label: "复制文件 ID", onClick: () => void navigator.clipboard?.writeText(file.id) }
  ];
  return (
    <RowActions
      primary={<Button icon={hasRevealToken ? <FolderOpenOutlined /> : <DownloadOutlined />} type="primary" size="small" loading={workingId === file.id} disabled={!canDownload || Boolean(workingId)} onClick={() => onDecrypt(file)}>
        {decryptActionText(file)}
      </Button>}
      detail={() => onDetail(file)}
      menu={menuItems}
      loading={workingId === file.id}
      disabled={Boolean(workingId)}
    />
  );
}

function TypographyUnknown() {
  return <span className="unknown-value">未知</span>;
}

function tableColumnSchema(_scope: FileCenterScope): Array<{ key: string; title: string }> {
  return [
    { key: "file", title: "文件名" },
    { key: "owner", title: "文件拥有者" },
    { key: "algorithm", title: "加密算法" },
    { key: "ciphertextSize", title: "密文大小" },
    { key: "createdAt", title: "创建时间" },
    { key: "actions", title: "下载操作" }
  ];
}

function emptyTextForScope(scope: FileCenterScope) {
  return scope === "owned_by_me" ? "你还没有创建加密文件" : "当前租户还没有可浏览的密文";
}
