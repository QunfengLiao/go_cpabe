import { describe, expect, it } from "vitest";
import { authorizationFormKind } from "./AlgorithmAuthorizationForm";

describe("算法授权表单注册表", () => {
  it("将首期 RSA 授权类型分派到 RSA 表单", () => {
    expect(authorizationFormKind("RSA_RECIPIENT")).toBe("rsa");
  });

  it("未知或空授权类型安全禁用", () => {
    expect(authorizationFormKind("TKN20_POLICY")).toBe("unsupported");
    expect(authorizationFormKind()).toBe("unsupported");
  });
});
