import { describe, expect, it } from "vitest";
import { belongsToRSAKeyScope, isUnsafeLinuxStorageBackend } from "./rsaKeyStore";

describe("RSA 私钥安全存储后端", () => {
  it("拒绝 Linux basic_text 和 unknown", () => {
    expect(isUnsafeLinuxStorageBackend("linux", "basic_text")).toBe(true);
    expect(isUnsafeLinuxStorageBackend("linux", "unknown")).toBe(true);
    expect(isUnsafeLinuxStorageBackend("linux", "kwallet6")).toBe(false);
  });

  it("其他平台不套用 Linux 后端判断", () => {
    expect(isUnsafeLinuxStorageBackend("win32", "unknown")).toBe(false);
  });

  it("账号和租户共同组成私钥索引隔离边界", () => {
    const record = { accountId: "7", tenantId: "3" };
    expect(belongsToRSAKeyScope(record, "7", "3")).toBe(true);
    expect(belongsToRSAKeyScope(record, "8", "3")).toBe(false);
    expect(belongsToRSAKeyScope(record, "7", "4")).toBe(false);
  });
});
