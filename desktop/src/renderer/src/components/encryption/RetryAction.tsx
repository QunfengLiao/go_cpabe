import { Alert, Button } from "antd";

export function retryGuidance(errorCode: string): { needsFile: boolean; message: string } {
  const needsFile = ["FILE_HANDLE_EXPIRED", "SOURCE_FILE_CHANGED"].includes(errorCode);
  return { needsFile, message: needsFile ? "本地文件句柄已失效，请重新选择原文件并创建新执行。" : `错误码：${errorCode}。重试会重新加密并创建新执行，不覆盖旧记录。` };
}

export function RetryAction({ errorCode, busy, onRetry }: { errorCode: string; busy: boolean; onRetry: () => void }) {
  const guidance = retryGuidance(errorCode);
  return <Alert type="error" showIcon message="本次执行未完成" description={guidance.message} action={<Button disabled={busy} onClick={onRetry}>{guidance.needsFile ? "重新选择文件" : "重试"}</Button>} />;
}
