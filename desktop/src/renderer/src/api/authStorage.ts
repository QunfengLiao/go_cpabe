import type { AuthStateSnapshot, CachedAccount, LoginData, RefreshData, TenantContextData, TenantRole, TenantSummary, User } from "../types";
import { clearAccessToken, clearLegacyRefreshTokens, getAccessToken as getAccessTokenFromStorage, saveAccessToken } from "./tokenStorage";
import { saveAccountSession } from "./authSessionStore";
import { cacheTenantBranding, mergeCachedBranding, normalizeTenantBranding } from "../theme/tenantBranding";

const USER_KEY = "go_cpabe_user";
const CURRENT_USER_ID_KEY = "go_cpabe_current_user_id";
const CURRENT_TENANT_ID_KEY = "go_cpabe_current_tenant_id";
const CURRENT_TENANT_CODE_KEY = "go_cpabe_current_tenant_code";
const LAST_TENANT_CODE_KEY = "go_cpabe_last_tenant_code";
const TENANTS_KEY = "go_cpabe_tenants";
const PLATFORM_ROLES_KEY = "go_cpabe_platform_roles";
const CACHED_ACCOUNTS_KEY = "go_cpabe_cached_accounts";
const TENANT_CONTEXTS_KEY = "go_cpabe_tenant_contexts";

export interface StoredUserTenantContext {
  userId: string;
  currentTenantId: string;
  currentTenantCode: string;
  tenants: TenantSummary[];
  tenantRoles: TenantRole[];
  platformRoles: TenantRole[];
  permissions: string[];
  updatedAt: number;
}

type LegacyCachedAccount = Partial<CachedAccount> & {
  refreshToken?: string;
  refreshTokenExpiresAt?: number;
  lastActiveAt?: number;
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

function toCachedAccount(user: User, platformRoles?: TenantRole[], status: CachedAccount["status"] = "active"): CachedAccount {
  return {
    userId: userIdOf(user),
    email: user.email,
    nickname: user.nickname,
    role: user.role,
    avatarUrl: user.avatar_url || undefined,
    platformRoles: normalizePlatformRoles(platformRoles),
    lastLoginAt: Date.now(),
    status,
    expired: status !== "active",
    loggedOut: false
  };
}

function normalizeTenant(tenant: TenantSummary): TenantSummary {
  const tenantId = tenant.tenant_id ?? tenant.tenantId ?? 0;
  const tenantName = tenant.tenant_name ?? tenant.tenantName ?? "";
  const tenantCode = tenant.tenant_code ?? tenant.tenantCode ?? "";
  return mergeCachedBranding({
    tenant_id: tenantId,
    tenant_name: tenantName,
    tenant_code: tenantCode,
    status: tenant.status,
    description: tenant.description,
    branding: normalizeTenantBranding(tenant.branding),
    roles: tenant.roles ?? [],
    user_count: tenant.user_count,
    tenant_admin_count: tenant.tenant_admin_count,
    created_at: tenant.created_at,
    updated_at: tenant.updated_at
  });
}

function normalizeTenantList(tenants?: TenantSummary[] | null): TenantSummary[] {
  return (tenants ?? []).map(normalizeTenant).filter((tenant) => tenant.tenant_id > 0);
}

function normalizeTenantRoles(roles?: TenantRole[] | null): TenantRole[] {
  return (roles ?? []).map((role) => String(role).trim()).filter(Boolean) as TenantRole[];
}

function emptyTenantContext(userId: string): StoredUserTenantContext {
  return {
    userId,
    currentTenantId: "",
    currentTenantCode: "",
    tenants: [],
    tenantRoles: [],
    platformRoles: [],
    permissions: [],
    updatedAt: Date.now()
  };
}

function readTenantContexts(): Record<string, StoredUserTenantContext> {
  const raw = localStorage.getItem(TENANT_CONTEXTS_KEY);
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as Record<string, StoredUserTenantContext>;
    if (!parsed || typeof parsed !== "object") return {};
    return Object.fromEntries(
      Object.entries(parsed).map(([userId, context]) => [
        userId,
        {
          ...emptyTenantContext(userId),
          ...context,
          userId,
          tenants: normalizeTenantList(context.tenants),
          tenantRoles: normalizeTenantRoles(context.tenantRoles),
          platformRoles: normalizePlatformRoles(context.platformRoles),
          permissions: Array.isArray(context.permissions) ? context.permissions.filter(Boolean) : []
        }
      ])
    );
  } catch {
    localStorage.removeItem(TENANT_CONTEXTS_KEY);
    return {};
  }
}

function writeTenantContexts(contexts: Record<string, StoredUserTenantContext>): void {
  localStorage.setItem(TENANT_CONTEXTS_KEY, JSON.stringify(contexts));
}

function clearLegacyGlobalTenantStorage(): void {
  localStorage.removeItem(CURRENT_TENANT_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_CODE_KEY);
  localStorage.removeItem(TENANTS_KEY);
  localStorage.removeItem(PLATFORM_ROLES_KEY);
}

export function migrateLegacyGlobalTenantStorage(userId: string | number): StoredUserTenantContext | null {
  const normalizedUserId = String(userId || "").trim();
  if (!normalizedUserId) return null;
  const contexts = readTenantContexts();
  if (contexts[normalizedUserId]) {
    clearLegacyGlobalTenantStorage();
    return contexts[normalizedUserId];
  }
  const legacyTenantsRaw = localStorage.getItem(TENANTS_KEY);
  const legacyTenantId = localStorage.getItem(CURRENT_TENANT_ID_KEY) ?? "";
  const legacyTenantCode = localStorage.getItem(CURRENT_TENANT_CODE_KEY) ?? "";
  const legacyPlatformRolesRaw = localStorage.getItem(PLATFORM_ROLES_KEY);
  if (!legacyTenantsRaw && !legacyTenantId && !legacyPlatformRolesRaw) return null;

  let tenants: TenantSummary[] = [];
  let platformRoles: TenantRole[] = [];
  try {
    tenants = normalizeTenantList(JSON.parse(legacyTenantsRaw || "[]") as TenantSummary[]);
  } catch {
    tenants = [];
  }
  try {
    platformRoles = normalizePlatformRoles(JSON.parse(legacyPlatformRolesRaw || "[]") as TenantRole[]);
  } catch {
    platformRoles = [];
  }
  const currentTenant = tenants.find((tenant) => String(tenant.tenant_id) === legacyTenantId);
  const context: StoredUserTenantContext = {
    userId: normalizedUserId,
    currentTenantId: currentTenant ? legacyTenantId : "",
    currentTenantCode: currentTenant ? legacyTenantCode || currentTenant.tenant_code : "",
    tenants,
    tenantRoles: currentTenant?.roles ?? [],
    platformRoles,
    permissions: [],
    updatedAt: Date.now()
  };
  contexts[normalizedUserId] = context;
  writeTenantContexts(contexts);
  clearLegacyGlobalTenantStorage();
  return context;
}

export function getStoredTenantContext(userId: string | number): StoredUserTenantContext | null {
  const normalizedUserId = String(userId || "").trim();
  if (!normalizedUserId) return null;
  const contexts = readTenantContexts();
  return contexts[normalizedUserId] ?? migrateLegacyGlobalTenantStorage(normalizedUserId);
}

export function saveStoredTenantContext(userId: string | number, context: Partial<StoredUserTenantContext>): StoredUserTenantContext {
  const normalizedUserId = String(userId || "").trim();
  const tenants = normalizeTenantList(context.tenants);
  tenants.forEach(cacheTenantBranding);
  const requestedTenantId = String(context.currentTenantId || "");
  const currentTenant = tenants.find((tenant) => String(tenant.tenant_id) === requestedTenantId) ?? null;
  const next: StoredUserTenantContext = {
    userId: normalizedUserId,
    currentTenantId: currentTenant ? String(currentTenant.tenant_id) : "",
    currentTenantCode: currentTenant ? context.currentTenantCode || currentTenant.tenant_code : "",
    tenants,
    tenantRoles: normalizeTenantRoles(context.tenantRoles ?? currentTenant?.roles ?? []),
    platformRoles: normalizePlatformRoles(context.platformRoles),
    permissions: Array.isArray(context.permissions) ? context.permissions.filter(Boolean) : [],
    updatedAt: Date.now()
  };
  const contexts = readTenantContexts();
  contexts[normalizedUserId] = next;
  writeTenantContexts(contexts);
  clearLegacyGlobalTenantStorage();
  return next;
}

export function clearStoredTenantContext(userId: string | number): void {
  const normalizedUserId = String(userId || "").trim();
  if (!normalizedUserId) return;
  const contexts = readTenantContexts();
  delete contexts[normalizedUserId];
  writeTenantContexts(contexts);
  clearLegacyGlobalTenantStorage();
}

export function getCurrentAccountTenantContext(): StoredUserTenantContext | null {
  return getStoredTenantContext(getCurrentUserId());
}

export function tenantContextFromAPI(userId: string | number, data: TenantContextData): StoredUserTenantContext {
  const normalizedUserId = String(userId || "").trim();
  const tenants = normalizeTenantList(data.tenants);
  const previous = getStoredTenantContext(normalizedUserId);
  const apiPlatformRoles = normalizePlatformRoles(data.platform_roles ?? data.platformRoles ?? []);
  const apiTenantId = data.current_tenant_id ? String(data.current_tenant_id) : "";
  const previousTenantId = previous?.currentTenantId ?? "";
  const selectedTenantId =
    tenants.length === 1
      ? String(tenants[0].tenant_id)
      : tenants.some((tenant) => String(tenant.tenant_id) === apiTenantId)
        ? apiTenantId
        : tenants.some((tenant) => String(tenant.tenant_id) === previousTenantId)
      ? previousTenantId
      : "";
  const currentTenant = tenants.find((tenant) => String(tenant.tenant_id) === selectedTenantId) ?? null;
  const context = saveStoredTenantContext(userId, {
    currentTenantId: currentTenant ? String(currentTenant.tenant_id) : "",
    currentTenantCode: currentTenant ? data.current_tenant_code ?? currentTenant.tenant_code : "",
    tenants,
    tenantRoles: data.tenantRoles?.length ? data.tenantRoles : currentTenant?.roles ?? [],
    platformRoles: apiPlatformRoles,
    permissions: data.permissions ?? []
  });
  const existingAccount = getCachedAccounts().find((account) => account.userId === normalizedUserId);
  if (existingAccount) {
    upsertCachedAccount({ ...existingAccount, platformRoles: apiPlatformRoles, status: "active", expired: false, loggedOut: false });
  } else if (data.user) {
    upsertCachedAccount(toCachedAccount(data.user, apiPlatformRoles, "active"));
  }
  return context;
}

function sanitizeCachedAccount(account: LegacyCachedAccount, userId = String(account.userId ?? "")): CachedAccount {
  const status = account.status ?? (account.expired || account.loggedOut ? "expired" : account.refreshToken ? "login_required" : "login_required");
  return {
    userId,
    email: String(account.email ?? ""),
    nickname: String(account.nickname || account.email || "未命名账号"),
    role: account.role ?? "data_user",
    avatarUrl: account.avatarUrl,
    platformRoles: normalizePlatformRoles(account.platformRoles),
    lastLoginAt: Number(account.lastLoginAt ?? account.lastActiveAt ?? Date.now()),
    status,
    expired: status !== "active",
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
    accounts[index] = { ...accounts[index], ...nextAccount };
  } else {
    accounts.push(nextAccount);
  }
  accounts.sort((left, right) => right.lastLoginAt - left.lastLoginAt);
  saveCachedAccounts(accounts);
  return accounts;
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
  clearLegacyRefreshTokens();
  const raw = localStorage.getItem(CACHED_ACCOUNTS_KEY);
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw) as LegacyCachedAccount[];
    if (!Array.isArray(parsed)) return [];
    const accounts = parsed
      .filter((account) => account.userId && account.email)
      .map((account) => sanitizeCachedAccount(account, String(account.userId)));
    saveCachedAccounts(accounts);
    return accounts;
  } catch {
    return [];
  }
}

export function getCurrentUserId(): string {
  return localStorage.getItem(CURRENT_USER_ID_KEY) ?? "";
}

export function getCurrentTenantId(): string {
  return getCurrentAccountTenantContext()?.currentTenantId ?? "";
}

export function getCurrentTenantCode(): string {
  return getCurrentAccountTenantContext()?.currentTenantCode ?? "";
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
  return getCurrentAccountTenantContext()?.tenants ?? [];
}

export function getCurrentTenant(): TenantSummary | null {
  const context = getCurrentAccountTenantContext();
  const currentTenantId = context?.currentTenantId ?? "";
  if (!currentTenantId) return null;
  return context?.tenants.find((tenant) => String(tenant.tenant_id) === currentTenantId) ?? null;
}

export function getStoredPlatformRoles(): TenantRole[] {
  return getCurrentAccountTenantContext()?.platformRoles ?? [];
}

export function saveCurrentTenant(tenantId: number | string, tenantCode?: string | null): void {
  const userId = getCurrentUserId();
  if (!userId) return;
  const current = getStoredTenantContext(userId) ?? emptyTenantContext(userId);
  const next = saveStoredTenantContext(userId, { ...current, currentTenantId: String(tenantId || ""), currentTenantCode: tenantCode ?? "" });
  if (next.currentTenantCode) localStorage.setItem(LAST_TENANT_CODE_KEY, next.currentTenantCode);
}

export function saveTenantContext(tenants: TenantSummary[], currentTenantId?: number | string | null, currentTenantCode?: string | null): void {
  const userId = getCurrentUserId();
  if (!userId) return;
  const normalizedTenants = normalizeTenantList(tenants);
  const tenant = normalizedTenants.find((item) => String(item.tenant_id) === String(currentTenantId ?? ""));
  const previous = getStoredTenantContext(userId);
  const next = saveStoredTenantContext(userId, {
    currentTenantId: tenant ? String(tenant.tenant_id) : "",
    currentTenantCode: tenant ? currentTenantCode ?? tenant.tenant_code : "",
    tenants: normalizedTenants,
    tenantRoles: tenant?.roles ?? previous?.tenantRoles ?? [],
    platformRoles: previous?.platformRoles ?? [],
    permissions: previous?.permissions ?? []
  });
  if (next.currentTenantCode) localStorage.setItem(LAST_TENANT_CODE_KEY, next.currentTenantCode);
}

export function getCurrentCachedAccount(): CachedAccount | null {
  const currentUserId = getCurrentUserId();
  return getCachedAccounts().find((account) => account.userId === currentUserId) ?? null;
}

export function getAuthSnapshot(): AuthStateSnapshot {
  const currentUserId = getCurrentUserId() || (getStoredUser() ? userIdOf(getStoredUser() as User) : "");
  const tenantContext = currentUserId ? getStoredTenantContext(currentUserId) : null;
  const currentTenant = tenantContext?.currentTenantId
    ? tenantContext.tenants.find((tenant) => String(tenant.tenant_id) === tenantContext.currentTenantId) ?? null
    : null;
  return {
    currentUserId,
    accessToken: getAccessToken(),
    user: getStoredUser(),
    tenants: tenantContext?.tenants ?? [],
    currentTenantId: tenantContext?.currentTenantId ?? "",
    currentTenantCode: tenantContext?.currentTenantCode ?? "",
    currentTenant,
    tenantRoles: tenantContext?.tenantRoles ?? currentTenant?.roles ?? [],
    permissions: tenantContext?.permissions ?? [],
    platformRoles: normalizePlatformRoles(tenantContext?.platformRoles),
    cachedAccounts: getCachedAccounts(),
    authReady: true,
    tenantContextReady: true,
    authStatus: currentUserId && !tenantContext?.currentTenantId && (tenantContext?.tenants.length ?? 0) > 1 ? "select-tenant" : "ready"
  };
}

export function getAccessToken(): string {
  return getAccessTokenFromStorage();
}

export function saveTokens(accessToken: string): void {
  saveAccessToken(accessToken);
}

export async function saveLoginSession(data: LoginData): Promise<AuthStateSnapshot> {
  const expiresAt = refreshTokenExpiresAt(data.refresh_token_expires_in);
  const tenants = normalizeTenantList(data.tenants);
  const platformRoles = normalizePlatformRoles(data.platform_roles ?? data.platformRoles);
  const account = toCachedAccount(data.user, platformRoles, "active");
  const currentTenantID = data.current_tenant_id ?? data.currentTenantId ?? null;
  const currentTenantCode =
    data.current_tenant_code ??
    data.currentTenantCode ??
    tenants.find((tenant) => tenant.tenant_id === currentTenantID)?.tenant_code ??
    null;

  await saveAccountSession(account.userId, data.refresh_token, expiresAt);
  saveTokens(data.access_token);
  saveUser(data.user);
  localStorage.setItem(CURRENT_USER_ID_KEY, account.userId);
  const tenantContext = saveStoredTenantContext(account.userId, {
    currentTenantId: currentTenantID ? String(currentTenantID) : "",
    currentTenantCode: currentTenantCode ?? "",
    tenants,
    tenantRoles: data.tenantRoles ?? tenants.find((tenant) => tenant.tenant_id === currentTenantID)?.roles ?? [],
    platformRoles,
    permissions: data.permissions ?? []
  });
  if (tenantContext.currentTenantCode) localStorage.setItem(LAST_TENANT_CODE_KEY, tenantContext.currentTenantCode);
  const cachedAccounts = upsertCachedAccount(account);
  return {
    currentUserId: account.userId,
    accessToken: data.access_token,
    user: data.user,
    tenants: tenantContext.tenants,
    currentTenantId: tenantContext.currentTenantId,
    currentTenantCode: tenantContext.currentTenantCode,
    currentTenant: tenantContext.currentTenantId ? tenantContext.tenants.find((tenant) => String(tenant.tenant_id) === tenantContext.currentTenantId) ?? data.currentTenant ?? null : null,
    tenantRoles: tenantContext.tenantRoles,
    permissions: tenantContext.permissions,
    platformRoles,
    cachedAccounts,
    authReady: true,
    tenantContextReady: true,
    authStatus: tenantContext.currentTenantId ? "ready" : tenantContext.tenants.length > 1 ? "select-tenant" : tenantContext.tenants.length === 0 ? "no-tenant" : "ready"
  };
}

export function saveRefreshedSession(account: CachedAccount, data: RefreshData, user?: User | null): AuthStateSnapshot {
  const previousUserId = getCurrentUserId();
  const nextUserId = user ? userIdOf(user) : account.userId;
  saveTokens(data.access_token);
  if (user) saveUser(user);
  if (!user && previousUserId && previousUserId !== nextUserId) saveUser(null);
  localStorage.setItem(CURRENT_USER_ID_KEY, nextUserId);
  const existingTenantContext = getStoredTenantContext(nextUserId) ?? emptyTenantContext(nextUserId);
  const platformRoles = normalizePlatformRoles(account.platformRoles ?? existingTenantContext.platformRoles);
  const cachedAccounts = upsertCachedAccount({
    ...account,
    userId: nextUserId,
    email: user?.email ?? account.email,
    nickname: user?.nickname ?? account.nickname,
    role: user?.role ?? account.role,
    avatarUrl: user?.avatar_url || account.avatarUrl,
    platformRoles,
    lastLoginAt: Date.now(),
    status: "active",
    expired: false,
    loggedOut: false
  });
  return {
    currentUserId: nextUserId,
    accessToken: data.access_token,
    user: user ?? getStoredUser(),
    tenants: existingTenantContext.tenants,
    currentTenantId: existingTenantContext.currentTenantId,
    currentTenantCode: existingTenantContext.currentTenantCode,
    currentTenant: existingTenantContext.currentTenantId ? existingTenantContext.tenants.find((tenant) => String(tenant.tenant_id) === existingTenantContext.currentTenantId) ?? null : null,
    tenantRoles: existingTenantContext.tenantRoles,
    permissions: existingTenantContext.permissions,
    platformRoles,
    cachedAccounts,
    authReady: true,
    tenantContextReady: Boolean(existingTenantContext.updatedAt),
    authStatus: "ready"
  };
}

export function saveUser(user: User | null): void {
  if (user) {
    const userId = userIdOf(user);
    const existingAccount = getCachedAccounts().find((account) => account.userId === userId);
    localStorage.setItem(USER_KEY, JSON.stringify(user));
    localStorage.setItem(CURRENT_USER_ID_KEY, userId);
    upsertCachedAccount(toCachedAccount(user, existingAccount?.platformRoles));
  } else {
    localStorage.removeItem(USER_KEY);
  }
}

export function markCachedAccountExpired(userId: string): CachedAccount[] {
  const accounts = getCachedAccounts().map((account) =>
    account.userId === userId ? { ...account, status: "expired" as const, expired: true, loggedOut: false } : account
  );
  saveCachedAccounts(accounts);
  return accounts;
}

export function markCachedAccountLoggedOut(userId: string): CachedAccount[] {
  const accounts = getCachedAccounts().map((account) =>
    account.userId === userId ? { ...account, status: "login_required" as const, expired: true, loggedOut: true } : account
  );
  saveCachedAccounts(accounts);
  return accounts;
}

export function removeCachedAccount(userId: string): CachedAccount[] {
  const accounts = getCachedAccounts().filter((account) => account.userId !== userId);
  saveCachedAccounts(accounts);
  return accounts;
}

export function clearCurrentSession(): void {
  clearAccessToken();
  localStorage.removeItem(USER_KEY);
  localStorage.removeItem(CURRENT_USER_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_ID_KEY);
  localStorage.removeItem(CURRENT_TENANT_CODE_KEY);
  localStorage.removeItem(TENANTS_KEY);
  localStorage.removeItem(PLATFORM_ROLES_KEY);
  clearSessionAuthState();
}

export function clearTenantStartupSession(): void {
  clearCurrentSession();
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
  clearLegacyRefreshTokens();
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
