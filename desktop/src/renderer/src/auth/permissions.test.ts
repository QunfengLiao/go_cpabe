import { describe, expect, it } from "vitest";
import { hasAllPermissionsInState, hasAnyPermissionInState, hasPermissionInState, uniquePermissions } from "./permissions";

const readyState = {
  authorizationStatus: "ready" as const,
  authorizationUserId: "1",
  authorizationTenantId: "101",
  currentUserId: "1",
  currentTenantId: "101",
  permissions: ["tenant.role.read", "tenant.member.manage"]
};

describe("permissions", () => {
  it("授权 ready 且 user/tenant 匹配时才允许命中权限", () => {
    expect(hasPermissionInState(readyState, "tenant.role.read")).toBe(true);
    expect(hasPermissionInState({ ...readyState, authorizationStatus: "loading" }, "tenant.role.read")).toBe(false);
    expect(hasPermissionInState({ ...readyState, authorizationUserId: "2" }, "tenant.role.read")).toBe(false);
    expect(hasPermissionInState({ ...readyState, authorizationTenantId: "202" }, "tenant.role.read")).toBe(false);
  });

  it("支持任一权限和全部权限判断", () => {
    expect(hasAnyPermissionInState(readyState, ["policy.write", "tenant.member.manage"])).toBe(true);
    expect(hasAnyPermissionInState(readyState, ["policy.write", "policy.publish"])).toBe(false);
    expect(hasAllPermissionsInState(readyState, ["tenant.role.read", "tenant.member.manage"])).toBe(true);
    expect(hasAllPermissionsInState(readyState, ["tenant.role.read", "policy.write"])).toBe(false);
    expect(hasAllPermissionsInState({ ...readyState, authorizationStatus: "error" }, [])).toBe(true);
  });

  it("清理空权限和重复权限", () => {
    expect(uniquePermissions([" tenant.role.read ", "", "tenant.role.read", "policy.read"])).toEqual(["tenant.role.read", "policy.read"]);
  });
});

