import type { EncryptedFileRecord } from "../api/encryption";

interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

type RevealTokenMap = Record<string, string>;

const STORAGE_PREFIX = "go-cpabe:decryption-reveal-tokens:v1";

export function browserRevealTokenStorage(): StorageLike | null {
  return typeof window !== "undefined" && window.localStorage ? window.localStorage : null;
}

export function loadLocalDecryptionRevealTokens(storage: StorageLike | null, tenantId: string, accountId: string): RevealTokenMap {
  if (!storage || !tenantId || !accountId) return {};
  try {
    const parsed = JSON.parse(storage.getItem(storageKey(tenantId, accountId)) ?? "{}") as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return Object.fromEntries(Object.entries(parsed).filter(([, token]) => typeof token === "string" && token.length > 0)) as RevealTokenMap;
  } catch {
    return {};
  }
}

export function saveLocalDecryptionRevealToken(storage: StorageLike | null, tenantId: string, accountId: string, fileId: string, revealToken?: string): void {
  if (!storage || !tenantId || !accountId || !fileId || !revealToken) return;
  const tokens = loadLocalDecryptionRevealTokens(storage, tenantId, accountId);
  tokens[fileId] = revealToken;
  storage.setItem(storageKey(tenantId, accountId), JSON.stringify(tokens));
}

export function removeLocalDecryptionRevealToken(storage: StorageLike | null, tenantId: string, accountId: string, fileId: string): void {
  if (!storage || !tenantId || !accountId || !fileId) return;
  const tokens = loadLocalDecryptionRevealTokens(storage, tenantId, accountId);
  delete tokens[fileId];
  const key = storageKey(tenantId, accountId);
  if (Object.keys(tokens).length === 0) {
    storage.removeItem(key);
    return;
  }
  storage.setItem(key, JSON.stringify(tokens));
}

export function applyLocalDecryptionRevealTokens<T extends EncryptedFileRecord>(items: T[], storage: StorageLike | null, tenantId: string, accountId: string): T[] {
  const tokens = loadLocalDecryptionRevealTokens(storage, tenantId, accountId);
  return items.map((item) => tokens[item.id] ? { ...item, local_decryption_reveal_token: tokens[item.id] } : item);
}

function storageKey(tenantId: string, accountId: string): string {
  return `${STORAGE_PREFIX}:${encodeURIComponent(tenantId)}:${encodeURIComponent(accountId)}`;
}
