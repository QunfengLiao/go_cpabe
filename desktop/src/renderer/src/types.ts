export type UserRole = "admin" | "data_owner" | "data_user";
export type UserStatus = "active" | "disabled";

export interface User {
  id: number;
  username?: string;
  email: string;
  nickname: string;
  phone?: string;
  role: UserRole;
  avatar_url: string;
  bio: string;
  birthday: string | null;
  must_change_password: boolean;
  status?: UserStatus;
  created_at: string;
  updated_at?: string;
}

export type BuiltinTenantRole = "PLATFORM_ADMIN" | "TENANT_ADMIN" | "DO" | "DU";
export type TenantRole = BuiltinTenantRole | (string & {});
export type TenantBusinessRole = "DATA_OWNER" | "DATA_VISITOR";
export type TenantStatus = "enabled" | "disabled";
export type AuthorizationStatus = "idle" | "loading" | "ready" | "error";
export type PermissionScopeType = "PLATFORM" | "TENANT";
export type PermissionStatus = "ACTIVE" | "DISABLED";
export type RoleScopeType = "PLATFORM" | "TENANT";
export type TenantRoleCategory = "GOVERNANCE" | "BUSINESS" | "CAPABILITY";
export type TenantRoleStatus = "ACTIVE" | "DISABLED";

export interface TenantBranding {
  logoUrl?: string;
  loginBackgroundUrl?: string;
  workspaceBackgroundUrl?: string;
  primaryColor?: string;
  sidebarColor?: string;
  backgroundStart?: string;
  backgroundEnd?: string;
  backgroundGlow?: string;
}

export interface TenantSummary {
  tenant_id: number;
  tenant_name: string;
  tenant_code: string;
  tenantId?: number;
  tenantName?: string;
  tenantCode?: string;
  status?: TenantStatus;
  description?: string;
  branding?: TenantBranding;
  roles?: TenantRole[];
  user_count?: number;
  tenant_admin_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface ApiEnvelope<T> {
  code: string | number;
  message?: string;
  msg?: string;
  data: T;
  request_id?: string;
}

export interface LoginData {
  access_token: string;
  access_token_expires_in: number;
  refresh_token: string;
  refresh_token_expires_in: number;
  token_type: string;
  user: User;
  current_tenant_id?: number | null;
  currentTenantId?: number | null;
  current_tenant_code?: string | null;
  currentTenantCode?: string | null;
  tenants?: TenantSummary[];
  currentTenant?: TenantSummary | null;
  tenantRoles?: TenantRole[];
  permissions?: string[];
  platform_roles?: TenantRole[];
  platformRoles?: TenantRole[];
}

export interface PermissionDTO {
  id: number;
  code: string;
  name: string;
  description?: string;
  scopeType: PermissionScopeType;
  resourceType: string;
  action: string;
  status: PermissionStatus;
}

export interface TenantRoleDTO {
  id: number;
  tenantId: number;
  code: string;
  name: string;
  description?: string;
  scopeType: RoleScopeType;
  roleCategory?: TenantRoleCategory;
  category?: TenantRoleCategory;
  builtin?: boolean;
  isBuiltin?: boolean;
  is_builtin?: boolean;
  status: TenantRoleStatus;
  permissionCount?: number;
  activeMemberCount?: number;
  createdAt?: string;
  created_at?: string;
  updatedAt?: string;
  updated_at?: string;
}

export interface RolePermissionDTO {
  roleId: number;
  permissionCodes: string[];
  permissions?: PermissionDTO[];
}

export interface MemberRoleDTO {
  tenantId: number;
  userId: number;
  roles: TenantRoleDTO[];
  permissions: string[];
}

export interface AuthorizationContextDTO {
  tenantId: number;
  roles: TenantRoleDTO[];
  permissions: string[];
}

export interface CreateTenantRoleInput {
  code: string;
  name: string;
  description?: string;
  permissionCodes: string[];
}

export interface UpdateTenantRoleInput {
  name: string;
  description?: string;
}

export interface DisableTenantRoleResult {
  roleId: number;
  status: "DISABLED";
  affectedMemberCount: number;
}

export interface SwitchTenantData {
  current_tenant_id: number;
  currentTenant?: TenantSummary;
  tenant?: TenantSummary;
  tenantRoles?: TenantRole[];
  permissions?: string[];
  roles?: TenantRole[];
  menus?: unknown[];
}

export interface TenantContextData {
  user?: User;
  current_tenant_id?: number | null;
  current_tenant_code?: string | null;
  currentTenant?: TenantSummary | null;
  tenantRoles?: TenantRole[];
  permissions?: string[];
  platform_roles?: TenantRole[];
  platformRoles?: TenantRole[];
  tenants: TenantSummary[];
}

export interface RefreshData {
  access_token: string;
  access_token_expires_in: number;
  refresh_token?: string;
  refresh_token_expires_in?: number;
  token_type: string;
}

export interface CachedAccount {
  userId: string;
  email: string;
  nickname: string;
  role: UserRole;
  avatarUrl?: string;
  platformRoles?: TenantRole[];
  lastLoginAt: number;
  status?: "active" | "expired" | "login_required";
  expired?: boolean;
  loggedOut?: boolean;
}

export interface AuthStateSnapshot {
  currentUserId: string;
  accessToken: string;
  user: User | null;
  tenants: TenantSummary[];
  currentTenantId: string;
  currentTenantCode: string;
  currentTenant: TenantSummary | null;
  tenantRoles: TenantRole[];
  permissions: string[];
  platformRoles: TenantRole[];
  cachedAccounts: CachedAccount[];
  authReady: boolean;
  tenantContextReady: boolean;
  authStatus: "idle" | "initializing" | "switching-account" | "switching-tenant" | "resolving-tenant" | "ready" | "no-tenant" | "select-tenant" | "error";
}

export interface TenantMember {
  user_id: number;
  username?: string;
  email: string;
  nickname: string;
  phone?: string;
  member_status: "active" | "disabled";
  roles: TenantRole[];
  joined_at?: string;
  created_at?: string;
}

export interface PlatformDashboard {
  tenant_count: number;
  enabled_tenant_count: number;
  disabled_tenant_count: number;
  user_count: number;
  tenant_user_count: number;
  tenant_admin_count: number;
  audit_enabled: boolean;
}

export interface TenantAdminAssignment {
  tenant_id: number;
  user_id: number;
  role: "TENANT_ADMIN";
  assigned?: boolean;
  removed?: boolean;
  created_user?: boolean;
  temporary_password?: string;
  user?: User;
}
