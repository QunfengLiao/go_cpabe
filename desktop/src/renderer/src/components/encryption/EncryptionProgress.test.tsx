import { describe, expect, it } from "vitest";
import { progressPresentation } from "./EncryptionProgress";

describe("真实加密进度展示", () => {
  it("只在字节可计算阶段显示百分比", () => {
    expect(progressPresentation("ENCRYPTING_FILE", 50, 100)).toMatchObject({ showPercent: true, percent: 50 });
    expect(progressPresentation("SAVING_METADATA", 0, 0)).toMatchObject({ showPercent: false, percent: undefined, label: "提交元数据" });
  });

  it("DEK 保护阶段使用真实接收者数量计算进度", () => {
    expect(progressPresentation("PROTECTING_KEY", 3, 10)).toMatchObject({
      showPercent: true,
      percent: 30,
      detail: "正在为第 3 / 10 位接收者保护 DEK"
    });
  });

  it("失败和取消使用明确阶段文案", () => {
    expect(progressPresentation("FAILED", 0, 100).label).toBe("失败");
    expect(progressPresentation("CANCELLED", 0, 100).label).toBe("已取消");
  });

  it("完成阶段固定显示 100%，不再把已完成显示为处理中", () => {
    expect(progressPresentation("COMPLETED", 0, 0)).toMatchObject({ showPercent: true, percent: 100, label: "上传完成" });
  });

  it("优先显示跨阶段统一的全局进度，而不是让每个阶段重新从 0 开始", () => {
    expect(progressPresentation("UPLOADING", 1, 100, 72)).toMatchObject({ showPercent: true, percent: 72 });
    expect(progressPresentation("SAVING_METADATA", 0, 0, 95)).toMatchObject({ showPercent: true, percent: 95 });
  });
});
