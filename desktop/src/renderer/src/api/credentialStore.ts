function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

function credentialStore(): DesktopCredentialStore | null {
  return window.desktopCredentialStore ?? null;
}

export function isCredentialStoreAvailable(): boolean {
  return Boolean(credentialStore());
}

export async function saveCredential(email: string, password: string): Promise<void> {
  const store = credentialStore();
  if (!store) throw new Error("当前运行环境不支持安全凭据存储");
  await store.saveCredential(normalizeEmail(email), password);
}

export async function getSavedEmails(): Promise<string[]> {
  const store = credentialStore();
  if (!store) return [];
  const emails = await store.getSavedEmails();
  return emails.map(normalizeEmail).filter(Boolean);
}

export async function getCredentialByEmail(email: string): Promise<string> {
  const store = credentialStore();
  if (!store) return "";
  return store.getCredentialByEmail(normalizeEmail(email));
}

export async function removeCredential(email: string): Promise<void> {
  const store = credentialStore();
  if (!store) return;
  await store.removeCredential(normalizeEmail(email));
}
