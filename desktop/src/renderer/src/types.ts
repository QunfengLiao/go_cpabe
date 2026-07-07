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
export type TenantStatus = "enabled" | "disabled";

export interface TenantSummary {
  tenant_id: number;
  tenant_name: string;
  tenant_code: string;
  tenantId?: number;
  tenantName?: string;
  tenantCode?: string;
  status?: TenantStatus;
  roles?: TenantRole[];
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
  lastLoginAt: number;
  expired?: boolean;
  loggedOut?: boolean;
}

export interface AuthStateSnapshot {
  currentUserId: string;
  accessToken: string;
  refreshToken: string;
  user: User | null;
  cachedAccounts: CachedAccount[];
}
