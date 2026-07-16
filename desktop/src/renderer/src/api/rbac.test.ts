import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  createTenantRole,
  disableTenantRole,
  getCurrentAuthorization,
  getTenantMemberRoles,
  getTenantRole,
  getTenantRolePermissions,
  listTenantPermissions,
  listTenantRoles,
  replaceTenantMemberRoles,
  replaceTenantRolePermissions,
  rbacErrorMessage,
  updateTenantRole
} from "./rbac";
import { ApiError, request } from "./request";

vi.mock("./request", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./request")>();
  return {
    ...actual,
    request: vi.fn()
  };
});

const requestMock = vi.mocked(request);

describe("rbac API client", () => {
  beforeEach(() => {
    requestMock.mockReset();
  });

  it("按当前租户 RBAC 路径读取授权和列表", async () => {
    requestMock
      .mockResolvedValueOnce({ tenantId: 1, roles: [], permissions: ["tenant.role.read"] })
      .mockResolvedValueOnce({ items: [{ id: 1, code: "tenant.role.read" }] })
      .mockResolvedValueOnce({ items: [{ id: 2, code: "DO" }] });

    await expect(getCurrentAuthorization()).resolves.toMatchObject({ tenantId: 1 });
    await expect(listTenantPermissions()).resolves.toHaveLength(1);
    await expect(listTenantRoles()).resolves.toHaveLength(1);

    expect(requestMock).toHaveBeenNthCalledWith(1, "/tenant/me/authorization", { cacheKey: "tenant:me:authorization" });
    expect(requestMock).toHaveBeenNthCalledWith(2, "/tenant/permissions");
    expect(requestMock).toHaveBeenNthCalledWith(3, "/tenant/roles");
  });

  it("创建、编辑和禁用角色使用契约请求体", async () => {
    requestMock.mockResolvedValue({ id: 15, code: "SRE_ENGINEER" });

    await createTenantRole({ code: "SRE_ENGINEER", name: "SRE 工程师", description: "稳定性", permissionCodes: ["policy.read"] });
    await updateTenantRole(15, { name: "高级 SRE 工程师", description: "稳定性和应急响应" });
    await disableTenantRole(15);

    expect(requestMock).toHaveBeenNthCalledWith(1, "/tenant/roles", {
      method: "POST",
      body: JSON.stringify({ code: "SRE_ENGINEER", name: "SRE 工程师", description: "稳定性", permissionCodes: ["policy.read"] })
    });
    expect(requestMock).toHaveBeenNthCalledWith(2, "/tenant/roles/15", {
      method: "PATCH",
      body: JSON.stringify({ name: "高级 SRE 工程师", description: "稳定性和应急响应" })
    });
    expect(requestMock).toHaveBeenNthCalledWith(3, "/tenant/roles/15", { method: "DELETE" });
  });

  it("角色权限和成员角色使用全量替换接口", async () => {
    requestMock.mockResolvedValue({});

    await getTenantRole(15);
    await getTenantRolePermissions(15);
    await replaceTenantRolePermissions(15, ["tenant.member.read", "policy.read"]);
    await getTenantMemberRoles(7);
    await replaceTenantMemberRoles(7, ["DO", "DU", "SRE_ENGINEER"]);

    expect(requestMock).toHaveBeenNthCalledWith(1, "/tenant/roles/15");
    expect(requestMock).toHaveBeenNthCalledWith(2, "/tenant/roles/15/permissions");
    expect(requestMock).toHaveBeenNthCalledWith(3, "/tenant/roles/15/permissions", {
      method: "PUT",
      body: JSON.stringify({ permissionCodes: ["tenant.member.read", "policy.read"] })
    });
    expect(requestMock).toHaveBeenNthCalledWith(4, "/tenant/members/7/roles");
    expect(requestMock).toHaveBeenNthCalledWith(5, "/tenant/members/7/roles", {
      method: "PUT",
      body: JSON.stringify({ roleCodes: ["DO", "DU", "SRE_ENGINEER"] })
    });
  });

  it("常见 RBAC 错误码映射为明确中文提示", () => {
    expect(rbacErrorMessage(new ApiError("duplicate", 409, "ROLE_CODE_EXISTS"))).toBe("角色编码已存在");
    expect(rbacErrorMessage(new ApiError("forbidden", 403, "PERMISSION_DENIED"))).toBe("权限不足，当前账号无法执行该操作");
    expect(rbacErrorMessage(new ApiError("原始错误", 500, "UNKNOWN"))).toBe("原始错误");
  });
});
