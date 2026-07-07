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
