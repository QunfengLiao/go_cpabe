import { Button, Card, Space, Tag, Typography, Upload } from "antd";
import { CheckCircleOutlined, FileOutlined, InboxOutlined } from "@ant-design/icons";
import { formatDateTime, formatFileSize } from "../../utils/fileDisplay";

export interface SelectedEncryptionFile {
  handleId: string;
  name: string;
  size: number;
  displayMimeType: string;
  lastModifiedMs: number;
}

export function FileSelector({ file, disabled, error, onSelect, onRemove }: { file: SelectedEncryptionFile | null; disabled?: boolean; error?: string; onSelect: () => void; onRemove: () => void }) {
  return (
    <Card title="1. 选择文件" className="encryption-card">
      {file ? (
        <div className="selected-file-card">
          <div className="selected-file-icon"><FileOutlined /></div>
          <div className="selected-file-main">
            <Typography.Text strong ellipsis={{ tooltip: file.name }}>{file.name}</Typography.Text>
            <div className="selected-file-meta">
              <span>{formatFileSize(file.size)}</span>
              <span>{file.displayMimeType || "未知类型"}</span>
              <span>修改于 {formatDateTime(new Date(file.lastModifiedMs).toISOString())}</span>
            </div>
            <Tag color="success" icon={<CheckCircleOutlined />}>已通过本地校验</Tag>
          </div>
          <Space>
            <Button onClick={onSelect} disabled={disabled}>更换</Button>
            <Button danger onClick={onRemove} disabled={disabled}>移除</Button>
          </Space>
        </div>
      ) : (
        <Upload.Dragger
          className="file-picker-dragger"
          disabled={disabled}
          showUploadList={false}
          openFileDialogOnClick={false}
          beforeUpload={() => false}
          onDrop={() => onSelect()}
        >
          <p className="ant-upload-drag-icon"><InboxOutlined /></p>
          <p className="ant-upload-text">点击选择本地文件</p>
          <p className="ant-upload-hint">明文文件路径只保存在桌面端上下文中，不会上传到服务端。</p>
          <Button type="primary" onClick={onSelect} disabled={disabled}>选择文件</Button>
        </Upload.Dragger>
      )}
      {error && <Typography.Text type="danger">{error}</Typography.Text>}
    </Card>
  );
}
