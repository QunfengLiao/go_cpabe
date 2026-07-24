import { beforeEach, describe, expect, it, vi } from "vitest";
import { createTenantMember, listTenantMembers } from "./tenant";
import { request } from "./request";

vi.mock("./request", () => ({ request: vi.fn() }));

describe("租户成员创建 API", () => {
  beforeEach(() => vi.mocked(request).mockReset());

  it("使用可信当前租户路径并提交 DO/DU 表单", async () => {
    vi.mocked(request).mockResolvedValue({ member: {}, created_user: true });
    await createTenantMember({ username: "new.du", displayName: "新成员", email: "new@example.com", phone: "13800000000", roles: ["DU", "DO"] });
    expect(request).toHaveBeenCalledWith("/tenant/members", expect.objectContaining({
      method: "POST",
      headers: expect.objectContaining({ "Idempotency-Key": expect.any(String) }),
      body: JSON.stringify({ username: "new.du", display_name: "新成员", email: "new@example.com", phone: "13800000000", roles: ["DU", "DO"] })
    }));
  });

  it("成员列表使用服务端分页并返回总数", async () => {
    vi.mocked(request).mockResolvedValue({ users: [{ user_id: 7 }], total: 10004, page: 3, page_size: 50 });
    const result = await listTenantMembers(2, 3, 50);
    expect(request).toHaveBeenCalledWith("/tenants/2/users?page=3&page_size=50");
    expect(result.total).toBe(10004);
    expect(result.users).toHaveLength(1);
  });
});
