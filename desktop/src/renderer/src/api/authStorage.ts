import type { AuthStateSnapshot, CachedAccount, LoginData, RefreshData, User } from "../types";

const ACCESS_TOKEN_KEY = "go_cpabe_access_token";
const REFRESH_TOKEN_KEY = "go_cpabe_refresh_token";
const USER_KEY = "go_cpabe_user";
const CURRENT_USER_ID_KEY = "go_cpabe_current_user_id";
const CACHED_ACCOUNTS_KEY = "go_cpabe_cached_accounts";

function userIdOf(user: Pick<User, "id">): string {
  return String(user.id);
}

function refreshTokenExpiresAt(expiresIn?: number): number | undefined {
  return expiresIn ? Date.now() + expiresIn * 1000 : undefined;
}

function toCachedAccount(user: User, refreshToken: string, expiresIn?: number): CachedAccount {
  return {
    userId: userIdOf(user),
    email: user.email,
    nickname: user.nickname,
    role: user.role,
    avatarUrl: user.avatar_url || undefined,
    user,
    refreshToken,
    refreshTokenExpiresAt: refreshTokenExpiresAt(expiresIn),
    lastActiveAt: Date.now(),
    expired: false,
    loggedOut: false
  };
}

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_TOKEN_KEY) ?? "";
}

export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_TOKEN_KEY) ?? "";
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
    const parsed = JSON.parse(raw) as CachedAccount[];
    return Array.isArray(parsed) ? parsed.filter((account) => account.userId && account.email) : [];
  } catch {
    return [];
  }
}

function saveCachedAccounts(accounts: CachedAccount[]): void {
  localStorage.setItem(CACHED_ACCOUNTS_KEY, JSON.stringify(accounts));
}

function upsertCachedAccount(nextAccount: CachedAccount): CachedAccount[] {
  const accounts = getCachedAccounts();
  const index = accounts.findIndex((account) => account.userId === nextAccount.userId);
  if (index >= 0) {
    accounts[index] = {
      ...accounts[index],
      ...nextAccount,
      refreshTokenExpiresAt: nextAccount.refreshTokenExpiresAt ?? accounts[index].refreshTokenExpiresAt
    };
  } else {
    accounts.push(nextAccount);
  }
  accounts.sort((left, right) => right.lastActiveAt - left.lastActiveAt);
  saveCachedAccounts(accounts);
  return accounts;
}

export function getCurrentUserId(): string {
  return localStorage.getItem(CURRENT_USER_ID_KEY) ?? "";
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
    cachedAccounts = upsertCachedAccount(toCachedAccount(user, refreshToken));
  }

  return {
    currentUserId,
    accessToken: getAccessToken(),
    refreshToken,
    user,
    cachedAccounts
  };
}

export function saveTokens(accessToken: string, refreshToken?: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  if (refreshToken !== undefined) {
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
  }
}

export function saveLoginSession(data: LoginData): AuthStateSnapshot {
  saveTokens(data.access_token, data.refresh_token);
  saveUser(data.user);
  const account = toCachedAccount(data.user, data.refresh_token, data.refresh_token_expires_in);
  localStorage.setItem(CURRENT_USER_ID_KEY, account.userId);
  const cachedAccounts = upsertCachedAccount(account);
  return {
    currentUserId: account.userId,
    accessToken: data.access_token,
    refreshToken: data.refresh_token,
    user: data.user,
    cachedAccounts
  };
}

export function saveRefreshedSession(account: CachedAccount, data: RefreshData, user?: User | null): AuthStateSnapshot {
  const nextUserId = user ? userIdOf(user) : account.userId;
  const nextRefreshToken = data.refresh_token ?? account.refreshToken;
  saveTokens(data.access_token, nextRefreshToken);
  if (user) saveUser(user);
  localStorage.setItem(CURRENT_USER_ID_KEY, nextUserId);

  const cachedAccounts = upsertCachedAccount({
    ...account,
    userId: nextUserId,
    email: user?.email ?? account.email,
    nickname: user?.nickname ?? account.nickname,
    role: user?.role ?? account.role,
    avatarUrl: user?.avatar_url || account.avatarUrl,
    user: user ?? account.user,
    refreshToken: nextRefreshToken,
    refreshTokenExpiresAt: refreshTokenExpiresAt(data.refresh_token_expires_in) ?? account.refreshTokenExpiresAt,
    lastActiveAt: Date.now(),
    expired: false,
    loggedOut: false
  });

  return {
    currentUserId: nextUserId,
    accessToken: data.access_token,
    refreshToken: nextRefreshToken,
    user: user ?? getStoredUser(),
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
      upsertCachedAccount(toCachedAccount(user, refreshToken));
    }
  } else {
    localStorage.removeItem(USER_KEY);
  }
}

export function markCachedAccountExpired(userId: string): CachedAccount[] {
  const accounts = getCachedAccounts().map((account) =>
    account.userId === userId ? { ...account, expired: true, loggedOut: false } : account
  );
  saveCachedAccounts(accounts);
  return accounts;
}

export function markCachedAccountLoggedOut(userId: string): CachedAccount[] {
  const accounts = getCachedAccounts().map((account) =>
    account.userId === userId ? { ...account, refreshToken: "", expired: true, loggedOut: true } : account
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
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
  localStorage.removeItem(CURRENT_USER_ID_KEY);
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
}
