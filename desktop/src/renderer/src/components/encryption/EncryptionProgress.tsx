import { Button, Card, Descriptions, Progress, Steps, Typography } from "antd";
import { formatDuration, formatFileSize } from "../../utils/fileDisplay";

const stages = ["VALIDATING", "ENCRYPTING_FILE", "PROTECTING_KEY", "UPLOADING", "SAVING_METADATA", "COMPLETED"];
const labels: Record<string, string> = { VALIDATING: "读取文件", ENCRYPTING_FILE: "AES-GCM 加密", PROTECTING_KEY: "RSA 封装 DEK", UPLOADING: "上传密文", SAVING_METADATA: "提交元数据", COMPLETED: "上传完成", FAILED: "失败", CANCELLED: "已取消" };

export function progressPresentation(stage: string, processedBytes: number, totalBytes: number, overallPercent?: number) {
  const current = Math.max(0, stages.indexOf(stage));
  const byteStage = ["ENCRYPTING_FILE", "UPLOADING"].includes(stage);
  const recipientStage = stage === "PROTECTING_KEY";
  const completed = stage === "COMPLETED";
  const showPercent = overallPercent != null || completed || (totalBytes > 0 && (byteStage || recipientStage));
  const percent = overallPercent != null
    ? Math.min(100, Math.max(0, Math.round(overallPercent)))
    : completed ? 100 : showPercent ? Math.min(100, Math.max(0, Math.round(processedBytes / totalBytes * 100))) : undefined;
  const detail = recipientStage && totalBytes > 0
    ? `正在为第 ${Math.min(processedBytes, totalBytes)} / ${totalBytes} 位接收者保护 DEK`
    : byteStage && totalBytes > 0
      ? `${formatFileSize(processedBytes)} / ${formatFileSize(totalBytes)}`
      : labels[stage] ?? stage;
  return { current, showPercent, percent, detail, label: labels[stage] ?? stage };
}

export function EncryptionProgress({ stage, processedBytes, totalBytes, percent, cancellable, startedAt, onCancel }: { stage: string; processedBytes: number; totalBytes: number; percent?: number; cancellable: boolean; startedAt?: number; onCancel: () => void }) {
  const presentation = progressPresentation(stage, processedBytes, totalBytes, percent);
  const elapsed = startedAt ? Date.now() - startedAt : undefined;
  return (
    <Card title="执行进度" className="encryption-card">
      <div className="encryption-progress-stack">
        <Steps size="small" current={presentation.current} status={stage === "FAILED" ? "error" : stage === "COMPLETED" ? "finish" : "process"} items={stages.map((item) => ({ title: labels[item] }))} />
        <div className="encryption-progress-main">
          <div>
            <Typography.Text strong>{presentation.label}</Typography.Text>
            <Typography.Paragraph type="secondary">{presentation.detail}</Typography.Paragraph>
          </div>
          {presentation.showPercent ? <Progress percent={presentation.percent} status={stage === "COMPLETED" ? "success" : undefined} /> : <Progress percent={100} status="active" showInfo={false} />}
        </div>
        <Descriptions size="small" column={{ xs: 1, sm: 2, md: 4 }} items={[
          { key: "processed", label: stage === "PROTECTING_KEY" ? "接收者进度" : "已处理", children: stage === "PROTECTING_KEY" && totalBytes > 0 ? `${processedBytes} / ${totalBytes} 人` : formatFileSize(processedBytes) },
          { key: "total", label: stage === "PROTECTING_KEY" ? "接收者总数" : "总大小", children: stage === "PROTECTING_KEY" && totalBytes > 0 ? `${totalBytes} 人` : formatFileSize(totalBytes) },
          { key: "elapsed", label: "累计耗时", children: formatDuration(elapsed) },
          { key: "percent", label: "总进度", children: presentation.percent == null ? "处理中" : `${presentation.percent}%` }
        ]} />
        {cancellable && <div className="encryption-progress-actions"><Button danger onClick={onCancel}>取消任务</Button></div>}
      </div>
    </Card>
  );
}
