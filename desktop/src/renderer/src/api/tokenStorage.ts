const ACCESS_TOKEN_KEY = "go_cpabe_access_token";
const REFRESH_TOKEN_KEY = "go_cpabe_refresh_token";
const ACCOUNT_REFRESH_TOKENS_KEY = "go_cpabe_account_refresh_tokens";

// 当前只是开发阶段的 Token 存储适配层：localStorage 不能提供系统级加密保护，
// 因此 refreshToken 不写入展示用账号缓存，而是集中放在这里，方便后续整体替换。
// 用户密码不得放在本模块或 localStorage；如需“记住密码”，只能通过独立 credentialStore 走系统级安全存储。
interface StoredAccountToken {
  refreshToken: string;
  expiresAt?: number;
}

type AccountTokenMap = Record<string, StoredAccountToken>;

function readAccountTokens(): AccountTokenMap {
  const raw = localStorage.getItem(ACCOUNT_REFRESH_TOKENS_KEY);
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw) as AccountTokenMap;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function writeAccountTokens(tokens: AccountTokenMap): void {
  localStorage.setItem(ACCOUNT_REFRESH_TOKENS_KEY, JSON.stringify(tokens));
}

function isExpired(expiresAt?: number): boolean {
  return Boolean(expiresAt && expiresAt <= Date.now());
}

export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_TOKEN_KEY) ?? "";
}

export function getRefreshToken(): string {
  return localStorage.getItem(REFRESH_TOKEN_KEY) ?? "";
}

export function saveTokens(accessToken: string, refreshToken?: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  if (refreshToken !== undefined) {
    localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
  }
}

export function clearCurrentTokens(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
}

export function saveAccountRefreshToken(userId: string, refreshToken: string, expiresAt?: number): void {
  if (!userId || !refreshToken) return;
  const tokens = readAccountTokens();
  tokens[userId] = { refreshToken, expiresAt };
  writeAccountTokens(tokens);
}

export function getAccountRefreshToken(userId: string): string {
  const tokens = readAccountTokens();
  const item = tokens[userId];
  if (!item?.refreshToken) return "";
  if (isExpired(item.expiresAt)) {
    delete tokens[userId];
    writeAccountTokens(tokens);
    return "";
  }
  return item.refreshToken;
}

export function hasAccountRefreshToken(userId: string): boolean {
  return Boolean(getAccountRefreshToken(userId));
}

export function removeAccountRefreshToken(userId: string): void {
  const tokens = readAccountTokens();
  if (!tokens[userId]) return;
  delete tokens[userId];
  writeAccountTokens(tokens);
}

export function clearAllTokens(): void {
  clearCurrentTokens();
  localStorage.removeItem(ACCOUNT_REFRESH_TOKENS_KEY);
}
