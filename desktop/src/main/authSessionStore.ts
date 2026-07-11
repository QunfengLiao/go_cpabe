import { app, safeStorage } from "electron";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";

interface StoredAuthSession {
  accountId: string;
  encryptedRefreshToken: string;
  expiresAt?: number;
  updatedAt: number;
}

interface AuthSessionFile {
  deviceId?: string;
  sessions?: Record<string, StoredAuthSession>;
}

export interface SessionRefreshResult {
  access_token: string;
  access_token_expires_in: number;
  token_type: string;
  refresh_token_expires_in?: number;
}

const SESSION_FILE = "auth-sessions.safe.json";
const DEFAULT_API_BASE_URL = "http://localhost:18080/api/v1";

function sessionPath(): string {
  return path.join(app.getPath("userData"), SESSION_FILE);
}

function apiBaseURL(): string {
  return (process.env.VITE_API_BASE_URL ?? process.env.API_BASE_URL ?? DEFAULT_API_BASE_URL).replace(/\/+$/, "");
}

function ensureSafeStorage(): void {
  if (!safeStorage.isEncryptionAvailable()) {
    throw new Error("当前系统安全存储不可用，无法保存登录会话");
  }
}

async function readSessionFile(): Promise<AuthSessionFile> {
  try {
    const raw = await fs.readFile(sessionPath(), "utf8");
    const parsed = JSON.parse(raw) as AuthSessionFile;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT") return {};
    throw error;
  }
}

async function writeSessionFile(file: AuthSessionFile): Promise<void> {
  await fs.mkdir(path.dirname(sessionPath()), { recursive: true });
  await fs.writeFile(sessionPath(), JSON.stringify(file, null, 2), "utf8");
}

function normalizeAccountId(accountId: string): string {
  return String(accountId || "").trim();
}

function encryptRefreshToken(refreshToken: string): string {
  ensureSafeStorage();
  return safeStorage.encryptString(refreshToken).toString("base64");
}

function decryptRefreshToken(encryptedRefreshToken: string): string {
  ensureSafeStorage();
  return safeStorage.decryptString(Buffer.from(encryptedRefreshToken, "base64"));
}

async function readRefreshToken(accountId: string): Promise<string> {
  const normalizedAccountId = normalizeAccountId(accountId);
  if (!normalizedAccountId) return "";
  const file = await readSessionFile();
  const session = file.sessions?.[normalizedAccountId];
  if (!session?.encryptedRefreshToken) return "";
  if (session.expiresAt && session.expiresAt <= Date.now()) {
    await removeAccountSession(normalizedAccountId);
    return "";
  }
  return decryptRefreshToken(session.encryptedRefreshToken);
}

export async function getDeviceID(): Promise<string> {
  const file = await readSessionFile();
  if (file.deviceId) return file.deviceId;
  file.deviceId = `electron-${crypto.randomBytes(16).toString("hex")}`;
  file.sessions ??= {};
  await writeSessionFile(file);
  return file.deviceId;
}

export async function saveAccountSession(accountId: string, refreshToken: string, expiresAt?: number): Promise<void> {
  const normalizedAccountId = normalizeAccountId(accountId);
  if (!normalizedAccountId || !refreshToken) return;
  const file = await readSessionFile();
  file.deviceId ??= `electron-${crypto.randomBytes(16).toString("hex")}`;
  file.sessions ??= {};
  file.sessions[normalizedAccountId] = {
    accountId: normalizedAccountId,
    encryptedRefreshToken: encryptRefreshToken(refreshToken),
    expiresAt,
    updatedAt: Date.now()
  };
  await writeSessionFile(file);
}

export async function hasAccountSession(accountId: string): Promise<boolean> {
  return Boolean(await readRefreshToken(accountId));
}

export async function removeAccountSession(accountId: string): Promise<void> {
  const normalizedAccountId = normalizeAccountId(accountId);
  if (!normalizedAccountId) return;
  const file = await readSessionFile();
  if (!file.sessions?.[normalizedAccountId]) return;
  delete file.sessions[normalizedAccountId];
  await writeSessionFile(file);
}

export async function refreshAccountSession(accountId: string): Promise<SessionRefreshResult> {
  const refreshToken = await readRefreshToken(accountId);
  if (!refreshToken) {
    throw new Error("账号没有可用的刷新会话");
  }
  const response = await fetch(`${apiBaseURL()}/auth/refresh`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken, device_id: await getDeviceID() })
  });
  const envelope = await response.json() as { data?: SessionRefreshResult & { refresh_token?: string; refresh_token_expires_in?: number }; message?: string; msg?: string };
  if (!response.ok || !envelope.data) {
    await removeAccountSession(accountId);
    throw new Error(envelope.message ?? envelope.msg ?? "刷新登录态失败");
  }
  if (envelope.data.refresh_token) {
    const expiresAt = envelope.data.refresh_token_expires_in ? Date.now() + envelope.data.refresh_token_expires_in * 1000 : undefined;
    await saveAccountSession(accountId, envelope.data.refresh_token, expiresAt);
  }
  return {
    access_token: envelope.data.access_token,
    access_token_expires_in: envelope.data.access_token_expires_in,
    refresh_token_expires_in: envelope.data.refresh_token_expires_in,
    token_type: envelope.data.token_type
  };
}

export async function logoutAccountSession(accountId: string): Promise<void> {
  const refreshToken = await readRefreshToken(accountId);
  try {
    if (refreshToken) {
      await fetch(`${apiBaseURL()}/auth/logout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken })
      });
    }
  } finally {
    await removeAccountSession(accountId);
  }
}
