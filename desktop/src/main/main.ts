import { app, BrowserWindow, dialog, ipcMain, shell } from "electron";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { getDeviceID, hasAccountSession, logoutAccountSession, refreshAccountSession, removeAccountSession, saveAccountSession } from "./authSessionStore";
import { getCredentialByEmail, getSavedEmails, removeCredential, saveCredential } from "./credentialStore";
import { createSanitizedExecutionError, EncryptionCoordinator } from "./encryption/coordinator";
import { clearDecryptionRevealTokens, decryptFile, rememberDecryptedFile, revealDecryptedFile, saveCiphertext, type PreservedCiphertextError } from "./encryption/decryptionCoordinator";
import { CryptoWorkerProcess } from "./encryption/cryptoWorkerProcess";
import { releaseAllFileHandles, selectSingleFile } from "./encryption/fileSelection";
import { requireAlgorithm, requireOpaqueFileHandle, requireTenantId, requireUUID, validateApiBaseUrl, validateIpcSender } from "./encryption/ipcValidation";
import { getRSAKeyStorageDirectory, listLocalRSAKeys, listPendingRSAKeys, markRSAKeyRegistered, saveGeneratedRSAKey, saveImportedRSAKey } from "./encryption/rsaKeyStore";
import { reportInterruptedExecutions, runEncryptionRecovery } from "./encryption/recovery";
import type { EncryptionExecutionDescriptor } from "./encryption/types";

const encryptionCoordinator = new EncryptionCoordinator();

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

function registerEncryptionHandlers(): void {
  ipcMain.handle("encryption:select-file", async (event) => { validateIpcSender(event); return selectSingleFile(); });
  ipcMain.handle("encryption:start", async (event, executionId: string, descriptor: EncryptionExecutionDescriptor) => {
    try {
      validateIpcSender(event);
      requireUUID(executionId, "execution");
      requireTenantId(descriptor.tenantId);
      requireOpaqueFileHandle(descriptor.fileHandleId);
      requireAlgorithm(descriptor.algorithmCode, descriptor.algorithmVersion);
      validateApiBaseUrl(descriptor.apiBaseUrl);
      if (!descriptor.accessToken || descriptor.accessToken.length > 8192 || !descriptor.accountId) throw new Error("ENCRYPTION_CONTEXT_INVALID");
      requireUUID(descriptor.authorization.rsaPublicKeyId, "rsa_public_key");
      if (!descriptor.recipients?.length) throw new Error("ENCRYPTION_RECIPIENTS_REQUIRED");
      for (const recipient of descriptor.recipients) {
        if (!recipient.userId || !recipient.publicKeyPem || !/^[0-9a-f]{64}$/i.test(recipient.publicKeyFingerprintSha256)) throw new Error("ENCRYPTION_RECIPIENT_INVALID");
        requireUUID(recipient.rsaPublicKeyId, "rsa_public_key");
      }
      return await encryptionCoordinator.start(event.sender, descriptor, executionId);
    } catch (error) {
      throw createSanitizedExecutionError(error);
    }
  });
  ipcMain.handle("encryption:cancel", async (event, executionId: string) => { validateIpcSender(event); await encryptionCoordinator.cancel(requireUUID(executionId, "execution")); });
  ipcMain.handle("encryption:clear-context", (event) => { validateIpcSender(event); encryptionCoordinator.cancelAll(); releaseAllFileHandles(); clearDecryptionRevealTokens(); });
  ipcMain.handle("encryption:rsa-generate", async (event, accountId: string, tenantId: string) => {
    validateIpcSender(event); requireTenantId(tenantId);
    const worker = new CryptoWorkerProcess();
    try { return saveGeneratedRSAKey(accountId, tenantId, await worker.generateRSAKey()); } finally { worker.terminate(); }
  });
  ipcMain.handle("encryption:rsa-import", async (event, accountId: string, tenantId: string) => {
    validateIpcSender(event); requireTenantId(tenantId);
    const choice = await dialog.showOpenDialog({ title: "导入 RSA 私钥", properties: ["openFile"], filters: [{ name: "PEM 私钥", extensions: ["pem", "key", "txt"] }] });
    if (choice.canceled || !choice.filePaths?.[0]) return null;
    let privateKeyPEM = await readFile(choice.filePaths[0], "utf8");
    try { return await saveImportedRSAKey(accountId, tenantId, privateKeyPEM, path.dirname(choice.filePaths[0])); } finally { privateKeyPEM = ""; }
  });
  ipcMain.handle("encryption:rsa-mark-registered", async (event, accountId: string, tenantId: string, fingerprint: string, keyId: string, version: number) => {
    validateIpcSender(event); requireTenantId(tenantId); requireUUID(keyId, "rsa_public_key");
    await markRSAKeyRegistered(accountId, tenantId, fingerprint, keyId, version);
  });
  ipcMain.handle("encryption:rsa-pending", async (event, accountId: string, tenantId: string) => { validateIpcSender(event); requireTenantId(tenantId); return listPendingRSAKeys(accountId, tenantId); });
  ipcMain.handle("encryption:rsa-local-keys", async (event, accountId: string, tenantId: string) => {
    validateIpcSender(event); requireTenantId(tenantId);
    return listLocalRSAKeys(accountId, tenantId);
  });
  ipcMain.handle("encryption:rsa-open-directory", async (event, accountId: string, tenantId: string) => {
    validateIpcSender(event); requireTenantId(tenantId);
    const directory = await getRSAKeyStorageDirectory(accountId, tenantId);
    if (!directory) return false;
    const error = await shell.openPath(directory);
    return !error;
  });
  const handleDecryptFile = async (event: Electron.IpcMainInvokeEvent, descriptor: { accountId: string; tenantId: string; apiBaseUrl: string; accessToken: string; fileId: string; suggestedFilename: string }) => {
    validateIpcSender(event); requireTenantId(descriptor.tenantId); requireUUID(descriptor.fileId, "received_file"); validateApiBaseUrl(descriptor.apiBaseUrl);
    if (!descriptor.accountId || !descriptor.accessToken) throw new Error("DECRYPTION_CONTEXT_INVALID");
    try {
      return await decryptFile(descriptor);
    } catch (error) {
      // 下载成功但本地解密失败是正常产品结果：将其转换为结构化 IPC 响应，确保 Renderer 能打开 .enc 所在目录。
      if (isPreservedCiphertextError(error)) {
        return {
          cancelled: false,
          decryptionFailed: true,
          preservedCiphertextFilename: error.preservedCiphertextFilename,
          revealToken: rememberDecryptedFile(error.preservedCiphertextPath),
          failureCode: error instanceof Error && "code" in error ? String((error as Error & { code?: unknown }).code ?? "LOCAL_DECRYPTION_FAILED") : "LOCAL_DECRYPTION_FAILED"
        };
      }
      throw error;
    }
  };
  ipcMain.handle("encryption:decrypt-file", handleDecryptFile);
  ipcMain.handle("encryption:decrypt-received", handleDecryptFile);
  ipcMain.handle("encryption:save-ciphertext", async (event, descriptor: { accountId: string; tenantId: string; apiBaseUrl: string; accessToken: string; fileId: string; suggestedFilename: string }) => {
    validateIpcSender(event); requireTenantId(descriptor.tenantId); requireUUID(descriptor.fileId, "received_file"); validateApiBaseUrl(descriptor.apiBaseUrl);
    if (!descriptor.accountId || !descriptor.accessToken) throw new Error("DECRYPTION_CONTEXT_INVALID");
    return saveCiphertext(descriptor);
  });
  ipcMain.handle("encryption:reveal-decrypted", async (event, revealToken: string) => {
    validateIpcSender(event);
    return revealDecryptedFile(revealToken);
  });
  ipcMain.handle("encryption:report-interrupted", async (event, accountId: string, tenantId: string, apiBaseUrl: string, accessToken: string) => {
    validateIpcSender(event); requireTenantId(tenantId); validateApiBaseUrl(apiBaseUrl);
    if (!accountId || !accessToken) throw new Error("ENCRYPTION_CONTEXT_INVALID");
    return reportInterruptedExecutions(accountId, tenantId, apiBaseUrl, accessToken);
  });
}

// isPreservedCiphertextError 仅识别主进程内部生成的密文保留错误，避免把网络或鉴权错误误报为已保存密文。
function isPreservedCiphertextError(error: unknown): error is Error & PreservedCiphertextError {
  return Boolean(error && typeof error === "object" && "preservedCiphertextFilename" in error && "preservedCiphertextPath" in error);
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
  mainWindow.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
  mainWindow.webContents.on("will-navigate", (event, target) => {
    if (target !== mainWindow.webContents.getURL()) event.preventDefault();
  });

  const rendererDevUrl = process.env.ELECTRON_RENDERER_URL;
  if (!app.isPackaged && rendererDevUrl) {
    // 仅开发模式加载 Vite 服务，避免生产客户端被环境变量引导至非受信任页面。
    void mainWindow.loadURL(rendererDevUrl);
  } else {
    void mainWindow.loadFile(path.join(__dirname, "../renderer/index.html"));
  }
}

void app.whenReady().then(() => {
  registerCredentialHandlers();
  registerAuthSessionHandlers();
  registerEncryptionHandlers();
  void runEncryptionRecovery();
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

app.on("before-quit", () => {
  encryptionCoordinator.cancelAll();
  releaseAllFileHandles();
});
