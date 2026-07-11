import { app, BrowserWindow, ipcMain } from "electron";
import path from "node:path";
import { getDeviceID, hasAccountSession, logoutAccountSession, refreshAccountSession, removeAccountSession, saveAccountSession } from "./authSessionStore";
import { getCredentialByEmail, getSavedEmails, removeCredential, saveCredential } from "./credentialStore";

function registerCredentialHandlers(): void {
  ipcMain.handle("credential:save", (_event, email: string, password: string) => saveCredential(email, password));
  ipcMain.handle("credential:get-saved-emails", () => getSavedEmails());
  ipcMain.handle("credential:get-by-email", (_event, email: string) => getCredentialByEmail(email));
  ipcMain.handle("credential:remove", (_event, email: string) => removeCredential(email));
}

function registerAuthSessionHandlers(): void {
  ipcMain.handle("auth-session:device-id", () => getDeviceID());
  ipcMain.handle("auth-session:save", (_event, accountId: string, refreshToken: string, expiresAt?: number) => saveAccountSession(accountId, refreshToken, expiresAt));
  ipcMain.handle("auth-session:has", (_event, accountId: string) => hasAccountSession(accountId));
  ipcMain.handle("auth-session:refresh", (_event, accountId: string) => refreshAccountSession(accountId));
  ipcMain.handle("auth-session:logout", (_event, accountId: string) => logoutAccountSession(accountId));
  ipcMain.handle("auth-session:remove", (_event, accountId: string) => removeAccountSession(accountId));
}

function createWindow(): void {
  const mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 1280,
    minHeight: 760,
    title: "CP-ABE 加密文件共享系统",
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, "../preload/preload.js"),
      contextIsolation: true,
      nodeIntegration: false
    }
  });
  mainWindow.setMenuBarVisibility(false);

  void mainWindow.loadFile(path.join(__dirname, "../renderer/index.html"));
}

void app.whenReady().then(() => {
  registerCredentialHandlers();
  registerAuthSessionHandlers();
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
