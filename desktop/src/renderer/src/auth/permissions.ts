import type { AuthorizationStatus } from "../types";

export interface PermissionCheckState {
  authorizationStatus: AuthorizationStatus;
  authorizationUserId: string;
  authorizationTenantId: string;
  currentUserId: string;
  currentTenantId: string;
  permissions: string[];
}

export interface PermissionRequirement {
  permission?: string;
  anyPermissions?: string[];
  allPermissions?: string[];
}

export function canUsePermissions(state: PermissionCheckState): boolean {
  return (
    state.authorizationStatus === "ready" &&
    Boolean(state.currentUserId) &&
    Boolean(state.currentTenantId) &&
    state.authorizationUserId === state.currentUserId &&
    state.authorizationTenantId === state.currentTenantId
  );
}

export function hasPermissionInState(state: PermissionCheckState, code: string): boolean {
  if (!code || !canUsePermissions(state)) return false;
  return new Set(state.permissions).has(code);
}

export function hasAnyPermissionInState(state: PermissionCheckState, codes: string[]): boolean {
  if (!codes.length || !canUsePermissions(state)) return false;
  const permissionSet = new Set(state.permissions);
  return codes.some((code) => permissionSet.has(code));
}

export function hasAllPermissionsInState(state: PermissionCheckState, codes: string[]): boolean {
  if (!codes.length) return true;
  if (!canUsePermissions(state)) return false;
  const permissionSet = new Set(state.permissions);
  return codes.every((code) => permissionSet.has(code));
}

export function satisfiesPermissionRequirement(state: PermissionCheckState, requirement: PermissionRequirement): boolean {
  if (requirement.permission && !hasPermissionInState(state, requirement.permission)) return false;
  if (requirement.anyPermissions?.length && !hasAnyPermissionInState(state, requirement.anyPermissions)) return false;
  if (requirement.allPermissions?.length && !hasAllPermissionsInState(state, requirement.allPermissions)) return false;
  return Boolean(requirement.permission || requirement.anyPermissions?.length || requirement.allPermissions?.length)
    ? true
    : canUsePermissions(state);
}

export function uniquePermissions(permissions: string[]): string[] {
  return Array.from(new Set(permissions.map((permission) => permission.trim()).filter(Boolean)));
}
