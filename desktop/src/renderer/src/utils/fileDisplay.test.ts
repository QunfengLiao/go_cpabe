import { describe, expect, it } from "vitest";
import {
  fileStatusPresentation,
  formatDateTime,
  formatDuration,
  formatFileSize,
  localDecryptionDuration,
  shortHash,
  summarizeRecipients,
  toDecryptionMetrics,
  toEncryptionMetrics
} from "./fileDisplay";

describe("文件展示格式化", () => {
  it("按 B/KiB/MiB/GiB 动态显示文件大小，避免小文件显示 0.00 MiB", () => {
    expect(formatFileSize(512)).toBe("512 B");
    expect(formatFileSize(1536)).toBe("1.5 KiB");
    expect(formatFileSize(2 * 1024 * 1024)).toBe("2.0 MiB");
    expect(formatFileSize(3 * 1024 * 1024 * 1024)).toBe("3.0 GiB");
    expect(formatFileSize(undefined)).toBe("—");
  });

  it("将接口时间转成固定本地时间格式，不直接暴露 ISO 字符串", () => {
    expect(formatDateTime("2026-07-12T14:27:20.320+08:00")).toBe("2026-07-12 14:27:20");
    expect(formatDateTime("")).toBe("—");
  });

  it("按毫秒和秒格式化耗时，缺失值显示占位", () => {
    expect(formatDuration(26)).toBe("26 ms");
    expect(formatDuration(1280)).toBe("1.28 s");
    expect(formatDuration(Number.NaN)).toBe("—");
  });

  it("把文件状态映射为中文文案", () => {
    expect(fileStatusPresentation("AVAILABLE").label).toBe("可用");
    expect(fileStatusPresentation("UPLOADING").label).toBe("上传中");
  });

  it("接收者摘要不回退显示数字 ID", () => {
    expect(summarizeRecipients([{ display_name: "李雷" }, { display_name: "韩梅梅" }, { display_name: "王强" }])).toBe("李雷、韩梅梅等 3 人");
    expect(summarizeRecipients([{ user_id: 9 }])).toBe("—");
  });

  it("长哈希默认缩短显示", () => {
    expect(shortHash("a".repeat(64))).toBe(`${"a".repeat(12)}…${"a".repeat(8)}`);
    expect(shortHash("")).toBe("—");
  });

  it("加密性能只合并 AES 和 DEK 保护耗时，不混入上传时间", () => {
    const metrics = toEncryptionMetrics({ benchmark: { aes_encrypt_ms: 20, dek_protect_ms: 6, upload_ms: 999 } });
    expect(metrics.localEncryptionDurationMs).toBe(26);
  });

  it("解密性能未执行时显示未解密，成功后显示最近一次本地解密耗时", () => {
    expect(localDecryptionDuration({})).toBe("未解密");
    expect(localDecryptionDuration({ decryption_metrics: { local_decryption_duration_ms: 126 } })).toBe("126 ms");
    expect(toDecryptionMetrics({ decryption_metrics: { local_decryption_duration_ms: 1320 } }).localDecryptionDurationMs).toBe(1320);
  });
});
