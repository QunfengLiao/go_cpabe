import { describe, expect, it } from "vitest";
import { memberCreationSuccessMessage } from "./TenantMembersPage";

describe("租户成员创建结果提示", () => {
  it("新账号展示一次性初始密码和首次改密提示", () => {
    expect(memberCreationSuccessMessage({ created_user: true, temporary_password: "lqf999.." })).toContain("提示尽快修改密码");
  });

  it("复用账号明确不修改原密码和资料", () => {
    expect(memberCreationSuccessMessage({ created_user: false })).toBe("已有账号已加入当前租户，原密码和资料未被修改。");
  });
});
