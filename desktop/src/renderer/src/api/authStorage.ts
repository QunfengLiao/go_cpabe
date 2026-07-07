import type { AuthStateSnapshot, CachedAccount, LoginData, RefreshData, TenantRole, TenantSummary, User } from "../types";
import {
  clearAllTokens,
  clearCurrentTokens,
  getAccessToken as getAccessTokenFromStorage,
  getAccountRefreshToken,
  getRefreshToken as getRefreshTokenFromStorage,
  hasAccountRefreshToken,
  removeAccountRefreshToken,
  saveAccountRefreshToken,
  saveTokens as saveTokensToStorage
} from "./tokenStorage";

const USER_KEY = "go_cpabe_user";
const CURRENT_USER_ID_KEY = "go_cpabe_current_user_id";
const CURRENT_TENANT_ID_KEY = "go_cpabe_current_tenant_id";
const CURRENT_TENANT_CODE_KEY = "go_cpabe_current_tenant_code";
const LAST_TENANT_CODE_KEY = "go_cpabe_last_tenant_code";
const TENANTS_KEY = "go_cpabe_tenants";
const PLATFORM_ROLES_KEY = "go_cpabe_platform_roles";
const CACHED_ACCOUNTS_KEY = "go_cpabe_cached_accounts";

type LegacyCachedAccount = Partial<CachedAccount> & {
  refreshToken?: string;
  refreshTokenExpiresAt?: number;
  lastActiveAt?: number;
  user?: User;
};

function userIdOf(user: Pick<User, "id">): string {
  return String(user.id);
}

function refreshTokenExpiresAt(expiresIn?: number): number | undefined {
  return expiresIn ? Date.now() + expiresIn * 1000 : undefined;
}

function normalizePlatformRoles(roles?: TenantRole[]): TenantRole[] {
  return (roles ?? []).filter((role) => role === "PLATFORM_ADMIN");
}

function toCachedAccount(user: User, platformRoles?: TenantRole[]): CachedAccount {
  const account: CachedAccount = {
    userId: userIdOf(user),
    email: user.email,
    nickname: user.nickname,
    role: user.role,
    avatarUrl: user.avatar_url || undefined,
    lastLoginAt: Date.now(),
    expired: false,
    loggedOut: false
  };
  if (platformRoles) {
    account.platformRoles = normalizePlatformRoles(platformRoles);
  }
  return account;
}

function normalizeTenant(tenant: TenantSummary): TenantSummary {
  const tenantId = tenant.tenant_id ?? tenant.tenantId ?? 0;
  const tenantName = tenant.tenant_name ?? tenant.tenantName ?? "";
  const tenantCode = tenant.tenant_code ?? tenant.tenantCode ?? "";
  return {
    tenant_id: tenantId,
    tenant_name: tenantName,
    tenant_code: tenantCode,
    status: tenant.status,
    description: tenant.description,
    roles: tenant.roles ?? [],
    user_count: tenant.user_count,
    tenant_admin_count: tenant.tenant_admin_count,
    created_at: tenant.created_at,
    updated_at: tenant.updated_at
  };
}

export function getStoredUser(): User | null {
  const raw = localStorage.getItem(USER_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

export function getCachedAccounts(): CachedAccount[] {
  const raw = localStorage.getItem(CACHED_ACCOUNTS_KEY);
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as LegacyCachedAccount[];
    if (!Array.isArray(parsed)) return [];
    const accounts = parsed
      .filter((account) => account.userId && account.email)
      .map((account) => {
        const userId = String(account.userId);
        if (account.refreshToken && !account.expired && !account.loggedOut) {
          saveAccountRefreshToken(userId, account.refreshToken, account.refreshTokenExpiresAt);
        }
        if (account.expired || account.loggedOut) {
          removeAccountRefreshToken(userId);
        }
        return sanitizeCachedAccount(account, userId);
      });
    saveCachedAccounts(accounts);
    return accounts;
  } catch {
    return [];
  }
}

function sanitizeCachedAccount(account: LegacyCachedAccount, userId = String(account.userId ?? "")): CachedAccount {
  return {
    userId,
    email: String(account.email ?? ""),
    nickname: String(account.nickname || account.email || "未命名账号"),
    role: account.role ?? "data_user",
    avatarUrl: account.avatarUrl,
    platformRoles: normalizePlatformRoles(account.platformRoles),
    lastLoginAt: Number(account.lastLoginAt ?? account.lastActiveAt ?? Date.now()),
    expired: Boolean(account.expired),
    loggedOut: Boolean(account.loggedOut)
  };
}

function saveCachedAccounts(accounts: CachedAccount[]): void {
  localStorage.setItem(CACHED_ACCOUNTS_KEY, JSON.stringify(accounts.map((account) => sanitizeCachedAccount(account))));
}

function upsertCachedAccount(nextAccount: CachedAccount): CachedAccount[] {
  const accounts = getCachedAccounts();
  const index = accounts.findIndex((account) => account.userId === nextAccount.userId);
  if (index >= 0) {
    accounts[index] = {
      ...accounts[index],
      ...nextAccount
    };
  } else {
    accounts.push(nextAccount);
  }
  accounts.sort((left, right) => right.lastLoginAt - left.lastLoginAt);
  saveCachedAccounts(accounts);
  return accounts;
}

export function getCurrentUserId(): string {
  return localStorage.getItem(CURRENT_USER_ID_KEY) ?? "";
}

export function getCurrentTenantId(): string {
  return localStorage.getItem(CURRENT_TENANT_ID_KEY) ?? "";
}

export function getCurrentTenantCode(): string {
  return localStorage.getItem(CURRENT_TENANT_CODE_KEY) ?? "";
}

export function getLastTenantCode(): string {
  return localStorage.getItem(LAST_TENANT_CODE_KEY) ?? "";
}

export function saveLastTenantCode(tenantCode: string): void {
  const normalizedCode = tenantCode.trim();
  if (normalizedCode) {
    localStorage.setItem(LAST_TENANT_CODE_KEY, normalizedCode);
  } else {
    localStorage.removeItem(LAST_TENANT_CODE_KEY);
  }
}

export function getStoredTenants(): TenantSummary[] {
  const raw = localStorage.getItem(TENANTS_KEY);
  if (!raw) return [];
  try {
    const tenants = JSON.parse(raw) as TenantSummary[];
    return Array.isArray(tenants) ? tenants.map(normalizeTenant) : [];
  } catch {
    return [];
  }
}

export function getStoredPlatformRoles(): TenantRole[] {
  const raw = localStorage.getItem(PLATFORM_ROLES_KEY);
  if (!raw) return [];
  try {
    const roles = JSON.parse(raw) as TenantRole[];
    return Array.isArray(roles) ? roles.filter((role) => role === "PLATFORM_ADMIN") : [];
  } catch {
    return [];
  }
}

export function saveCurrentTenant(tenantId: number | string, tenantCode?: string | null): void {
  const normalizedId = String(tenantId || "");
  const normalizedCode = String(tenantCode || "").trim();
  if (normalizedId) {
    localStorage.setItem(CURRENT_TENANT_ID_KEY, normalizedId);
  } else {
    localStorage.removeItem(CURRENT_TENANT_ID_KEY);
  }
  if (normalizedCode) {
    localStorage.setItem(CURRENT_TENANT_CODE_KEY, normalizedCode);
    localStorage.setItem(LAST_TENANT_CODE_KEY, normalizedCode);
  }
}

export function getCurrentCachedAccount(): CachedAccount | null {
  const currentUserId = getCurrentUserId();
  return getCachedAccounts().find((account) => account.userId === currentUserId) ?? null;
}

export function getAuthSnapshot(): AuthStateSnapshot {
  const user = getStoredUser();
  const currentUserId = getCurrentUserId() || (user ? userIdOf(user) : "");
  const refreshToken = getRefreshToken();
  let cachedAccounts = getCachedAccounts();

  if (user && refreshToken && !cachedAccounts.some((account) => account.userId === userIdOf(user))) {
    saveAccountRefreshToken(userIdOf(user), refreshToken);
    cachedAccounts = upsertCachedAccount(toCachedAccount(user));
  }

  return {
    currentUserId,
    accessToken: getAccessToken(),
    refreshToken,
    user,
    tenants: getStoredTenants(),
    currentTenantId: getCurrentTenantId(),
    currentTenantCode: getCurrentTenantCode(),
    platformRoles: normalizePlatformRoles(getStoredPlatformRoles()),
    cachedAccounts
  };
}

export function getAccessToken(): string {
  return getAccessTokenFromStorage();
}

export function getRefreshToken(): string {
  return getRefreshTokenFromStorage();
}

export function saveTokens(accessToken: string, refreshToken?: string): void {
  saveTokensToStorage(accessToken, refreshToken);
}

export function saveLoginSession(data: LoginData): AuthStateSnapshot {
  const expiresAt = refreshTokenExpiresAt(data.refresh_token_expires_in);
  const tenants = (data.tenants ?? []).map(normalizeTenant);
  const platformRoles = normalizePlatformRoles(data.platform_roles ?? data.platformRoles);
  const account = toCachedAccount(data.user, platformRoles);
  const currentTenantID = data.current_tenant_id ?? data.currentTenantId ?? null;
  const currentTenantCode =
    data.current_tenant_code ??
    data.currentTenantCode ??
    tenants.find((tenant) => tenant.tenant_id === currentTenantID)?.tenant_code ??
    null;
  saveTokens(data.access_token, data.refresh_token);
  saveAccountRefreshToken(account.userId, data.refresh_token, expiresAt);
  saveUser(data.user);
  localStorage.setItem(TENANTS_KEY, JSON.stringify(tenants));
  localStorage.setItem(PLATFORM_ROLES_KEY, JSON.stringify(platformRoles));
  if (currentTenantID) {
    saveCurrentTenant(currentTenantID, currentTenantCode);
  } else {
    localStorage.removeItem(CURRENT_TENANT_ID_KEY);
    localStorage.removeItem(CURRENT_TENANT_CODE_KEY);
  }
  localStorage.setItem(CURRENT_USER_ID_KEY, account.userId);
  const cachedAccounts = upsertCachedAccount(account);
  return {
    currentUserId: account.userId,
    accessToken: data.access_token,
    refreshToken: data.refresh_token,
    user: data.user,
    tenants,
    currentTenantId: currentTenantID ? String(currentTenantID) : "",
    currentTenantCode: currentTenantCode ?? "",
    platformRoles,
    cachedAccounts
  };
}

export function saveRefreshedSession(account: CachedAccount, data: RefreshData, user?: User | null): AuthStateSnapshot {
  const previousUserId = getCurrentUserId();
  const nextUserId = user ? userIdOf(user) : account.userId;
  const nextRefreshToken = data.refresh_token ?? getAccountRefreshToken(account.userId) ?? getRefreshToken();
  const expiresAt = refreshTokenExpiresAt(data.refresh_token_expires_in);
  saveTokens(data.access_token, nextRefreshToken);
  saveAccountRefreshToken(nextUserId, nextRefreshToken, expiresAt);
  if (user) saveUser(user);
  if (!user && previousUserId && previousUserId !== nextUserId) saveUser(null);
  localStorage.setItem(CURRENT_USER_ID_KEY, nextUserId);
  const platformRoles = normalizePlatformRoles(account.platformRoles);
  // refresh 接口只轮换 token，不返回角色；多账号切换时必须使用账号自己的缓存角色，
  // 否则普通账号可能沿用上一个 Platform Admin 的本地菜单状态。真正授权仍由后端中间件判断。
  localStorage.setItem(PLATFORM_ROLES_KEY, JSON.stringify(platformRoles));

  const cachedAccounts = upsertCachedAccount({
    ...account,
    userId: nextUserId,
    email: user?.email ?? account.email,
    nickname: user?.nickname ?? account.nickname,
    role: user?.role ?? account.role,
    avatarUrl: user?.avatar_url || account.avatarUrl,
    platformRoles,
    lastLoginAt: Date.now(),
    expired: false,
    loggedOut: false
  });

  return {
    currentUserId: nextUserId,
    accessToken: data.access_token,
    refreshToken: nextRefreshToken,
    user: user ?? getStoredUser(),
    tenants: getStoredTenants(),
    currentTenantId: getCurrentTenantId(),
    currentTenantCode: getCurrentTenantCode(),
    platformRoles,
    cachedAccounts
  };
}

export function saveUser(user: User | null): void {
  if (user) {
    localStorage.setItem(USER_KEY, JSON.stringify(user));
    const currentUserId = localStorage.getItem(CURRENT_USER_ID_KEY);
    const refreshToken = getRefreshToken();
    if (refreshToken && (!currentUserId || currentUserId === userIdOf(user))) {
      localStorage.setItem(CURRENT_USER_ID_KEY, userIdOf(user));
      if (!hasAccountRefreshToken(userIdOf(user))) {
        saveAccountRefreshToken(userIdOf(user), refreshToken);
      }
      upsertCachedAccount(toCachedAccount(user));
    }
  } else {
    localStorage.removeItem(USER_KEY);
  }
}

export function getCachedAccountRefreshToken(userId: string): string {
  return getAccountRefreshToken(userId);
}

export function hasCachedAccountToken(userId: string): boolean {
  return hasAccountRefreshToken(userId);
}

export function markCachedAccountExpired(userId: string): CachedAccount[] {
  removeAccountRefreshToken(userId);
  const accounts = getCachedAccounts().map((account) =>
    account.userId === userId ? { ...account, expired: true, loggedOut: false } : account
  );
  saveCachedAccounts(accounts);
  return accounts;
}

export function markCachedAccountLoggedOut(userId: string): CachedAccount[] {
  removeAccountRefreshToken(userId);
  const accounts = getCachedAccounts().map((account) =>
    account.userId === userId ? { ...account, expired: true, loggedOut: true } : account
  );
  saveCachedAccounts(accounts);
  return accounts;
}

export function removeCachedAccount(userId: string): CachedAccount[] {
  removeAccountRefreshToken(userId);
  const accounts = getCachedAccounts().filter((account) => account.userId !== userId);
  saveCachedAccounts(accounts);
  return accounts;
}

export function clearCurrentSession(): void {
  clearCurrentTokens();
  localStorage.removeItem(USER_KEY);
  localStorage.removeItem(CURRENT_USER_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_CODE_KEY);
  localStorage.removeItem(TENANTS_KEY);
  localStorage.removeItem(PLATFORM_ROLES_KEY);
  clearSessionAuthState();
}

export function clearTenantStartupSession(): void {
  clearAllTokens();
  localStorage.removeItem(USER_KEY);
  localStorage.removeItem(CURRENT_USER_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_CODE_KEY);
  localStorage.removeItem(TENANTS_KEY);
  localStorage.removeItem(PLATFORM_ROLES_KEY);
  clearSessionAuthState();
}

export function expireCurrentSession(): CachedAccount[] {
  const currentUserId = getCurrentUserId();
  const accounts = currentUserId ? markCachedAccountExpired(currentUserId) : getCachedAccounts();
  clearCurrentSession();
  return accounts;
}

export function clearStoredAuth(): void {
  clearCurrentSession();
  localStorage.removeItem(CACHED_ACCOUNTS_KEY);
  clearAllTokens();
}

function clearSessionAuthState(): void {
  const keysToRemove = ["go_cpabe_last_route", "go_cpabe_login_state", "lastRoute"];
  for (const key of keysToRemove) {
    sessionStorage.removeItem(key);
  }
  for (let index = sessionStorage.length - 1; index >= 0; index -= 1) {
    const key = sessionStorage.key(index);
    if (key?.startsWith("go_cpabe_auth_") || key?.startsWith("go_cpabe_tenant_")) {
      sessionStorage.removeItem(key);
    }
  }
}
