import { describe, expect, it, vi } from "vitest";
import { ProgressBridge } from "./progressBridge";

function target() {
  return { isDestroyed: () => false, send: vi.fn() };
}

describe("进度桥接隔离", () => {
  it("拒绝字节倒退和账号租户串线", () => {
    const bridge = new ProgressBridge(); const webContents = target();
    bridge.publish(webContents as never, { executionId: "e1", accountId: "7", tenantId: "3", stage: "ENCRYPTING_FILE", processedBytes: 8, totalBytes: 10, cancellable: true });
    bridge.publish(webContents as never, { executionId: "e1", accountId: "7", tenantId: "3", stage: "ENCRYPTING_FILE", processedBytes: 7, totalBytes: 10, cancellable: true });
    bridge.publish(webContents as never, { executionId: "e1", accountId: "8", tenantId: "3", stage: "UPLOADING", processedBytes: 10, totalBytes: 10, cancellable: true });
    expect(webContents.send).toHaveBeenCalledTimes(1);
  });

  it("不同执行独立分发且完成后可重新使用执行 ID", () => {
    const bridge = new ProgressBridge(); const webContents = target();
    for (const executionId of ["e1", "e2"]) bridge.publish(webContents as never, { executionId, accountId: "7", tenantId: "3", stage: "VALIDATING", processedBytes: 0, totalBytes: 10, cancellable: true });
    bridge.finish("e1");
    bridge.publish(webContents as never, { executionId: "e1", accountId: "9", tenantId: "4", stage: "VALIDATING", processedBytes: 0, totalBytes: 10, cancellable: true });
    expect(webContents.send).toHaveBeenCalledTimes(3);
  });

  it("允许跨阶段切换到不同进度单位，避免丢弃接收者进度", () => {
    const bridge = new ProgressBridge(); const webContents = target();
    bridge.publish(webContents as never, { executionId: "e1", accountId: "7", tenantId: "3", stage: "ENCRYPTING_FILE", processedBytes: 1024, totalBytes: 1024, cancellable: true });
    bridge.publish(webContents as never, { executionId: "e1", accountId: "7", tenantId: "3", stage: "PROTECTING_KEY", processedBytes: 1, totalBytes: 10, cancellable: true });
    expect(webContents.send).toHaveBeenCalledTimes(2);
  });

  it("组合真实阶段进度并保持总体百分比单调，不生成随机进度", () => {
    const bridge = new ProgressBridge(); const webContents = target();
    const publish = (stage: "VALIDATING" | "ENCRYPTING_FILE" | "PROTECTING_KEY" | "UPLOADING" | "SAVING_METADATA" | "COMPLETED", processedBytes: number, totalBytes: number) => bridge.publish(webContents as never, { executionId: "e1", accountId: "7", tenantId: "3", stage, processedBytes, totalBytes, cancellable: true });
    publish("VALIDATING", 1, 1); publish("ENCRYPTING_FILE", 50, 100); publish("PROTECTING_KEY", 1, 4); publish("UPLOADING", 25, 100); publish("SAVING_METADATA", 0, 0); publish("COMPLETED", 1, 1);
    const events = webContents.send.mock.calls.map((call) => call[1] as { percent: number; protectedRecipients?: number; uploadedBytes?: number });
    expect(events.map((event) => event.percent)).toEqual([5, 30, 59, 76, 95, 100]);
    expect(events[2].protectedRecipients).toBe(1);
    expect(events[3].uploadedBytes).toBe(25);
  });
});
