import { app, safeStorage } from "electron";
import fs from "node:fs/promises";
import path from "node:path";

interface StoredCredential {
  email: string;
  encryptedPassword: string;
  updatedAt: number;
}

type CredentialFile = Record<string, StoredCredential>;

const CREDENTIAL_FILE = "credentials.safe.json";

function credentialPath(): string {
  return path.join(app.getPath("userData"), CREDENTIAL_FILE);
}

function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

async function readCredentialFile(): Promise<CredentialFile> {
  try {
    const raw = await fs.readFile(credentialPath(), "utf8");
    const parsed = JSON.parse(raw) as CredentialFile;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT") return {};
    throw error;
  }
}

async function writeCredentialFile(credentials: CredentialFile): Promise<void> {
  await fs.mkdir(path.dirname(credentialPath()), { recursive: true });
  await fs.writeFile(credentialPath(), JSON.stringify(credentials, null, 2), "utf8");
}

function ensureSafeStorage(): void {
  if (!safeStorage.isEncryptionAvailable()) {
    throw new Error("当前系统安全存储不可用，无法安全保存密码");
  }
}

export async function saveCredential(email: string, password: string): Promise<void> {
  const normalizedEmail = normalizeEmail(email);
  if (!normalizedEmail || !password) return;

  ensureSafeStorage();
  const credentials = await readCredentialFile();
  credentials[normalizedEmail] = {
    email: normalizedEmail,
    // 密码只在主进程进入系统级安全存储加密流程，渲染层和 localStorage 都不保存明文。
    encryptedPassword: safeStorage.encryptString(password).toString("base64"),
    updatedAt: Date.now()
  };
  await writeCredentialFile(credentials);
}

export async function getSavedEmails(): Promise<string[]> {
  const credentials = await readCredentialFile();
  return Object.values(credentials)
    .map((item) => item.email)
    .filter(Boolean)
    .sort((left, right) => left.localeCompare(right));
}

export async function getCredentialByEmail(email: string): Promise<string> {
  const normalizedEmail = normalizeEmail(email);
  if (!normalizedEmail) return "";

  ensureSafeStorage();
  const credentials = await readCredentialFile();
  const credential = credentials[normalizedEmail];
  if (!credential?.encryptedPassword) return "";
  return safeStorage.decryptString(Buffer.from(credential.encryptedPassword, "base64"));
}

export async function removeCredential(email: string): Promise<void> {
  const normalizedEmail = normalizeEmail(email);
  if (!normalizedEmail) return;

  const credentials = await readCredentialFile();
  if (!credentials[normalizedEmail]) return;
  delete credentials[normalizedEmail];
  await writeCredentialFile(credentials);
}
