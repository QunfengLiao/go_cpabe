import { describe, expect, it } from "vitest";
import { retryGuidance } from "./RetryAction";

describe("重试动作提示", () => {
  it("文件句柄失效要求重新选择", () => { expect(retryGuidance("FILE_HANDLE_EXPIRED").needsFile).toBe(true); });
  it("网络类错误提示创建新执行且不覆盖旧记录", () => { const guidance = retryGuidance("UPLOAD_INTERRUPTED"); expect(guidance.needsFile).toBe(false); expect(guidance.message).toContain("不覆盖旧记录"); });
});
