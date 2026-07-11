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
