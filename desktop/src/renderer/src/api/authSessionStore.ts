import type { RefreshData } from "../types";

function authSessionStore(): DesktopAuthSessionStore | null {
  return window.desktopAuthSessionStore ?? null;
}

export function isAuthSessionStoreAvailable(): boolean {
  return Boolean(authSessionStore());
}

export async function getDeviceId(): Promise<string> {
  const store = authSessionStore();
  if (!store) return "web-fallback";
  return store.getDeviceId();
}

export async function saveAccountSession(accountId: string, refreshToken: string, expiresAt?: number): Promise<void> {
  const store = authSessionStore();
  if (!store || !accountId || !refreshToken) return;
  await store.saveSession(accountId, refreshToken, expiresAt);
}

export async function hasAccountSession(accountId: string): Promise<boolean> {
  const store = authSessionStore();
  if (!store || !accountId) return false;
  return store.hasSession(accountId);
}

export async function refreshAccountSession(accountId: string): Promise<RefreshData> {
  const store = authSessionStore();
  if (!store) throw new Error("当前运行环境不支持安全会话存储");
  return store.refreshSession(accountId);
}

export async function logoutAccountSession(accountId: string): Promise<void> {
  const store = authSessionStore();
  if (!store || !accountId) return;
  await store.logoutSession(accountId);
}

export async function removeAccountSession(accountId: string): Promise<void> {
  const store = authSessionStore();
  if (!store || !accountId) return;
  await store.removeSession(accountId);
}
