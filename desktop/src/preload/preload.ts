import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("desktopRuntime", {
  platform: process.platform
});

contextBridge.exposeInMainWorld("desktopCredentialStore", {
  saveCredential: (email: string, password: string) => ipcRenderer.invoke("credential:save", email, password),
  getSavedEmails: () => ipcRenderer.invoke("credential:get-saved-emails"),
  getCredentialByEmail: (email: string) => ipcRenderer.invoke("credential:get-by-email", email),
  removeCredential: (email: string) => ipcRenderer.invoke("credential:remove", email)
});

contextBridge.exposeInMainWorld("desktopAuthSessionStore", {
  getDeviceId: () => ipcRenderer.invoke("auth-session:device-id"),
  saveSession: (accountId: string, refreshToken: string, expiresAt?: number) => ipcRenderer.invoke("auth-session:save", accountId, refreshToken, expiresAt),
  hasSession: (accountId: string) => ipcRenderer.invoke("auth-session:has", accountId),
  refreshSession: (accountId: string) => ipcRenderer.invoke("auth-session:refresh", accountId),
  logoutSession: (accountId: string) => ipcRenderer.invoke("auth-session:logout", accountId),
  removeSession: (accountId: string) => ipcRenderer.invoke("auth-session:remove", accountId)
});

contextBridge.exposeInMainWorld("desktopEncryption", {
  selectFile: () => ipcRenderer.invoke("encryption:select-file"),
  start: (executionId: string, descriptor: unknown) => ipcRenderer.invoke("encryption:start", executionId, descriptor),
  cancel: (executionId: string) => ipcRenderer.invoke("encryption:cancel", executionId),
  clearContext: () => ipcRenderer.invoke("encryption:clear-context"),
  generateRSAKey: (accountId: string, tenantId: string) => ipcRenderer.invoke("encryption:rsa-generate", accountId, tenantId),
  importPrivateKey: (accountId: string, tenantId: string) => ipcRenderer.invoke("encryption:rsa-import", accountId, tenantId),
  markRSAKeyRegistered: (accountId: string, tenantId: string, fingerprint: string, keyId: string, version: number) => ipcRenderer.invoke("encryption:rsa-mark-registered", accountId, tenantId, fingerprint, keyId, version),
  listPendingRSAKeys: (accountId: string, tenantId: string) => ipcRenderer.invoke("encryption:rsa-pending", accountId, tenantId),
  listLocalRSAKeys: (accountId: string, tenantId: string) => ipcRenderer.invoke("encryption:rsa-local-keys", accountId, tenantId),
  openRSAKeyDirectory: (accountId: string, tenantId: string) => ipcRenderer.invoke("encryption:rsa-open-directory", accountId, tenantId),
  decryptReceivedFile: (descriptor: unknown) => ipcRenderer.invoke("encryption:decrypt-received", descriptor),
  decryptFile: (descriptor: unknown) => ipcRenderer.invoke("encryption:decrypt-file", descriptor),
  saveCiphertext: (descriptor: unknown) => ipcRenderer.invoke("encryption:save-ciphertext", descriptor),
  revealDecryptedFile: (revealToken: string) => ipcRenderer.invoke("encryption:reveal-decrypted", revealToken),
  reportInterrupted: (accountId: string, tenantId: string, apiBaseUrl: string, accessToken: string) => ipcRenderer.invoke("encryption:report-interrupted", accountId, tenantId, apiBaseUrl, accessToken),
  onProgress: (listener: (event: unknown) => void) => {
    const wrapped = (_event: Electron.IpcRendererEvent, progress: unknown) => listener(progress);
    ipcRenderer.on("encryption:progress", wrapped);
    return () => ipcRenderer.removeListener("encryption:progress", wrapped);
  }
});
