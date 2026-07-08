export type UserRole = "admin" | "data_owner" | "data_user";
export type UserStatus = "active" | "disabled";

export interface User {
  id: number;
  email: string;
  nickname: string;
  role: UserRole;
  avatar_url: string;
  bio: string;
  birthday: string | null;
  status?: UserStatus;
  created_at: string;
  updated_at?: string;
}

export type TenantRole = "PLATFORM_ADMIN" | "TENANT_ADMIN" | "DO" | "DU";
export type TenantBusinessRole = "DATA_OWNER" | "DATA_VISITOR";
export type TenantStatus = "enabled" | "disabled";

export interface TenantSummary {
  tenant_id: number;
  tenant_name: string;
  tenant_code: string;
  tenantId?: number;
  tenantName?: string;
  tenantCode?: string;
  status?: TenantStatus;
  description?: string;
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
  platform_roles?: TenantRole[];
  platformRoles?: TenantRole[];
}

export interface SwitchTenantData {
  current_tenant_id: number;
  tenant?: TenantSummary;
  roles?: TenantRole[];
  menus?: unknown[];
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
  expired?: boolean;
  loggedOut?: boolean;
}

export interface AuthStateSnapshot {
  currentUserId: string;
  accessToken: string;
  refreshToken: string;
  user: User | null;
  tenants: TenantSummary[];
  currentTenantId: string;
  currentTenantCode: string;
  platformRoles: TenantRole[];
  cachedAccounts: CachedAccount[];
}

export interface TenantMember {
  user_id: number;
  email: string;
  nickname: string;
  member_status: "active" | "disabled";
  roles: TenantRole[];
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
}
