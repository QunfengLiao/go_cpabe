import type {
  AuthorizationContextDTO,
  CreateTenantRoleInput,
  DisableTenantRoleResult,
  MemberRoleDTO,
  PermissionDTO,
  RolePermissionDTO,
  TenantRoleDTO,
  UpdateTenantRoleInput
} from "../types";
import { ApiError, request } from "./request";

interface ItemsEnvelope<T> {
  items?: T[];
}

export const RBAC_ERROR_MESSAGES: Record<string, string> = {
  PERMISSION_DENIED: "权限不足，当前账号无法执行该操作",
  TENANT_PERMISSION_DENIED: "权限不足，当前账号无法执行该操作",
  ROLE_CODE_EXISTS: "角色编码已存在",
  ROLE_NOT_FOUND: "角色不存在或不属于当前租户",
  BUILTIN_ROLE_IMMUTABLE: "系统内置角色由系统管理，不允许修改",
  ROLE_DISABLED: "角色已禁用",
  INVALID_ROLE_SCOPE: "角色作用域非法",
  INVALID_PERMISSION_SCOPE: "权限作用域非法",
  CANNOT_ASSIGN_PLATFORM_ROLE: "不能在租户内分配平台角色",
  CANNOT_REMOVE_LAST_TENANT_ADMIN: "不能移除最后一名租户管理员",
  MEMBER_NOT_FOUND_IN_TENANT: "成员不属于当前租户或已不可用",
  CROSS_TENANT_ACCESS_DENIED: "当前租户范围内资源不存在或不可访问"
};

export function getCurrentAuthorization(): Promise<AuthorizationContextDTO> {
  return request("/tenant/me/authorization", { cacheKey: "tenant:me:authorization" });
}

export async function listTenantPermissions(): Promise<PermissionDTO[]> {
  const data = await request<ItemsEnvelope<PermissionDTO>>("/tenant/permissions");
  return data.items ?? [];
}

export async function listTenantRoles(): Promise<TenantRoleDTO[]> {
  const data = await request<ItemsEnvelope<TenantRoleDTO>>("/tenant/roles");
  return data.items ?? [];
}

export function createTenantRole(input: CreateTenantRoleInput): Promise<TenantRoleDTO> {
  return request("/tenant/roles", {
    method: "POST",
    body: JSON.stringify({
      code: input.code,
      name: input.name,
      description: input.description ?? "",
      permissionCodes: input.permissionCodes
    })
  });
}

export function getTenantRole(roleId: number): Promise<TenantRoleDTO> {
  return request(`/tenant/roles/${roleId}`);
}

export function updateTenantRole(roleId: number, input: UpdateTenantRoleInput): Promise<TenantRoleDTO> {
  return request(`/tenant/roles/${roleId}`, {
    method: "PATCH",
    body: JSON.stringify({
      name: input.name,
      description: input.description ?? ""
    })
  });
}

export function disableTenantRole(roleId: number): Promise<DisableTenantRoleResult> {
  return request(`/tenant/roles/${roleId}`, { method: "DELETE" });
}

export function getTenantRolePermissions(roleId: number): Promise<RolePermissionDTO> {
  return request(`/tenant/roles/${roleId}/permissions`);
}

export function replaceTenantRolePermissions(roleId: number, permissionCodes: string[]): Promise<RolePermissionDTO> {
  return request(`/tenant/roles/${roleId}/permissions`, {
    method: "PUT",
    body: JSON.stringify({ permissionCodes })
  });
}

export function getTenantMemberRoles(userId: number): Promise<MemberRoleDTO> {
  return request(`/tenant/members/${userId}/roles`);
}

export function replaceTenantMemberRoles(userId: number, roleIds: number[]): Promise<MemberRoleDTO> {
  return request(`/tenant/members/${userId}/roles`, {
    method: "PUT",
    body: JSON.stringify({ roleIds })
  });
}

export function rbacErrorMessage(error: unknown, fallback = "请求失败，请稍后重试"): string {
  if (!(error instanceof ApiError)) return error instanceof Error ? error.message : fallback;
  const code = String(error.code ?? "");
  return RBAC_ERROR_MESSAGES[code] ?? error.message ?? fallback;
}

export function isBuiltinTenantRole(role: Pick<TenantRoleDTO, "builtin" | "isBuiltin" | "is_builtin">): boolean {
  return Boolean(role.builtin ?? role.isBuiltin ?? role.is_builtin);
}

export function isTenantVisibleRole(role: TenantRoleDTO, tenantId?: number): boolean {
  if (role.scopeType !== "TENANT" || role.code === "PLATFORM_ADMIN") return false;
  if (role.tenantId === 0) return isBuiltinTenantRole(role);
  if (!tenantId) return false;
  return role.tenantId === tenantId;
}

export function filterTenantVisibleRoles(roles: TenantRoleDTO[], tenantId?: number): TenantRoleDTO[] {
  return roles.filter((role) => isTenantVisibleRole(role, tenantId));
}
