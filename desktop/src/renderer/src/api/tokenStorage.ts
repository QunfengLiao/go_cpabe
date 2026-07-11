const ACCESS_TOKEN_KEY = "go_cpabe_access_token";
const LEGACY_REFRESH_TOKEN_KEY = "go_cpabe_refresh_token";
const LEGACY_ACCOUNT_REFRESH_TOKENS_KEY = "go_cpabe_account_refresh_tokens";

// Renderer 只保存短期 Access Token；Refresh Token 必须由 Electron Main Process 的安全存储管理。
export function getAccessToken(): string {
  return localStorage.getItem(ACCESS_TOKEN_KEY) ?? "";
}

export function saveAccessToken(accessToken: string): void {
  if (accessToken) {
    localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  } else {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
  }
}

export function clearAccessToken(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
}

export function clearLegacyRefreshTokens(): void {
  localStorage.removeItem(LEGACY_REFRESH_TOKEN_KEY);
  localStorage.removeItem(LEGACY_ACCOUNT_REFRESH_TOKENS_KEY);
}
