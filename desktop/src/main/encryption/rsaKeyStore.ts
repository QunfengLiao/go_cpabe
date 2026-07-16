import { app, safeStorage } from "electron";
import { createHash, createPrivateKey, createPublicKey } from "node:crypto";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";

interface StoredRSAKey {
  accountId: string;
  tenantId: string;
  fingerprintSha256: string;
  publicKeyPem: string;
  privateKeyCiphertextBase64: string;
  publicKeyId?: string;
  version?: number;
  registrationPending: boolean;
  createdAt: string;
  /** 本地安全存储目录；只由主进程使用，Renderer 不会拿到该绝对路径。 */
  privateKeyDirectory?: string;
  /** 导入时的来源目录仅用于本地记录，绝不参与私钥读取和解密。 */
  sourceDirectory?: string;
}

export interface LocalRSAKeySummary {
  fingerprintSha256: string;
  publicKeyId?: string;
  version?: number;
  registrationPending: boolean;
  createdAt: string;
}

export interface RSAKeyRegistrationMaterial {
  publicKeyPem: string;
  fingerprintSha256: string;
  keyBits: number;
  algorithm: "RSA-OAEP-SHA256";
}

export function isUnsafeLinuxStorageBackend(platform: NodeJS.Platform, backend: string): boolean {
  return platform === "linux" && (backend === "basic_text" || backend === "unknown");
}

export function belongsToRSAKeyScope(record: { accountId: string; tenantId: string }, accountId: string, tenantId: string): boolean {
  return record.accountId === accountId && record.tenantId === tenantId;
}

export async function saveGeneratedRSAKey(accountId: string, tenantId: string, key: { publicKeyPem: string; privateKeyPem: string; fingerprintSha256: string }): Promise<RSAKeyRegistrationMaterial> {
  assertSafeStorage();
  const encrypted = safeStorage.encryptString(key.privateKeyPem);
  const records = await readRecords();
  const next: StoredRSAKey = { accountId, tenantId, fingerprintSha256: key.fingerprintSha256, publicKeyPem: key.publicKeyPem, privateKeyCiphertextBase64: encrypted.toString("base64"), registrationPending: true, createdAt: new Date().toISOString(), privateKeyDirectory: privateKeyStorageDirectory() };
  const filtered = records.filter((item) => !(belongsToRSAKeyScope(item, accountId, tenantId) && item.fingerprintSha256 === key.fingerprintSha256));
  filtered.push(next);
  await writeRecords(filtered);
  encrypted.fill(0);
  return { publicKeyPem: key.publicKeyPem, fingerprintSha256: key.fingerprintSha256, keyBits: 3072, algorithm: "RSA-OAEP-SHA256" };
}

// saveImportedRSAKey 从主进程读入的私钥派生公钥指纹，并立即使用 Electron safeStorage 加密保存私钥。
// 私钥正文只在本函数调用栈内短暂存在，Renderer 只能得到公钥登记材料。
export async function saveImportedRSAKey(accountId: string, tenantId: string, privateKeyPEM: string, sourceDirectory?: string): Promise<RSAKeyRegistrationMaterial> {
  assertSafeStorage();
  const privateKey = createPrivateKey(privateKeyPEM);
  const publicKey = createPublicKey(privateKey);
  const publicDer = publicKey.export({ type: "spki", format: "der" });
  const publicKeyPem = publicKey.export({ type: "spki", format: "pem" }).toString();
  const fingerprintSha256 = createHash("sha256").update(publicDer).digest("hex");
  const keyBits = Number((privateKey.asymmetricKeyDetails as { modulusLength?: number } | undefined)?.modulusLength ?? 3072);
  const encrypted = safeStorage.encryptString(privateKeyPEM);
  const records = await readRecords();
  const next: StoredRSAKey = { accountId, tenantId, fingerprintSha256, publicKeyPem, privateKeyCiphertextBase64: encrypted.toString("base64"), registrationPending: true, createdAt: new Date().toISOString(), privateKeyDirectory: privateKeyStorageDirectory(), sourceDirectory };
  const filtered = records.filter((item) => !(belongsToRSAKeyScope(item, accountId, tenantId) && item.fingerprintSha256 === fingerprintSha256));
  filtered.push(next);
  await writeRecords(filtered);
  encrypted.fill(0);
  return { publicKeyPem, fingerprintSha256, keyBits, algorithm: "RSA-OAEP-SHA256" };
}

export async function markRSAKeyRegistered(accountId: string, tenantId: string, fingerprintSha256: string, publicKeyId: string, version: number): Promise<void> {
  const records = await readRecords();
  const item = records.find((record) => belongsToRSAKeyScope(record, accountId, tenantId) && record.fingerprintSha256 === fingerprintSha256);
  if (!item) throw new Error("LOCAL_RSA_KEY_NOT_FOUND");
  item.publicKeyId = publicKeyId;
  item.version = version;
  item.registrationPending = false;
  await writeRecords(records);
}

export async function listPendingRSAKeys(accountId: string, tenantId: string): Promise<RSAKeyRegistrationMaterial[]> {
  const records = await readRecords();
  return records.filter((item) => belongsToRSAKeyScope(item, accountId, tenantId) && item.registrationPending).map((item) => ({ publicKeyPem: item.publicKeyPem, fingerprintSha256: item.fingerprintSha256, keyBits: 3072, algorithm: "RSA-OAEP-SHA256" }));
}

/** listLocalRSAKeys 返回当前账号和租户的本地密钥摘要，不暴露私钥、公钥正文或安全存储路径。 */
export async function listLocalRSAKeys(accountId: string, tenantId: string): Promise<LocalRSAKeySummary[]> {
  const records = await readRecords();
  return records
    .filter((item) => belongsToRSAKeyScope(item, accountId, tenantId))
    .map(({ fingerprintSha256, publicKeyId, version, registrationPending, createdAt }) => ({ fingerprintSha256, publicKeyId, version, registrationPending, createdAt }));
}

/** getRSAKeyStorageDirectory 返回当前租户的本地安全存储目录，仅供主进程打开目录使用。 */
export async function getRSAKeyStorageDirectory(accountId: string, tenantId: string): Promise<string | null> {
  const records = await readRecords();
  const record = records.find((item) => belongsToRSAKeyScope(item, accountId, tenantId));
  return record?.privateKeyDirectory ?? (record ? privateKeyStorageDirectory() : null);
}

// loadRSAPrivateKeyForDecryption 按账号、租户和公钥 UUID/指纹解封私钥，仅供主进程立即传给本地 Worker；
// 当服务端只提供指纹时允许省略 UUID，但仍必须完成指纹精确匹配。
export async function loadRSAPrivateKeyForDecryption(accountId: string, tenantId: string, publicKeyId: string, expectedFingerprint: string): Promise<string> {
  assertSafeStorage();
  const records = await readRecords();
  const item = records.find((record) => belongsToRSAKeyScope(record, accountId, tenantId) && (!publicKeyId || record.publicKeyId === publicKeyId) && record.fingerprintSha256 === expectedFingerprint && !record.registrationPending);
  if (!item) throw new Error("LOCAL_RSA_PRIVATE_KEY_NOT_FOUND");
  const encrypted = Buffer.from(item.privateKeyCiphertextBase64, "base64");
  try {
    return safeStorage.decryptString(encrypted);
  } finally {
    encrypted.fill(0);
  }
}

function assertSafeStorage(): void {
  if (!safeStorage.isEncryptionAvailable()) throw new Error("SAFE_STORAGE_UNAVAILABLE");
  if (process.platform === "linux") {
    const backend = safeStorage.getSelectedStorageBackend();
    if (isUnsafeLinuxStorageBackend(process.platform, backend)) throw new Error("SAFE_STORAGE_BACKEND_UNSAFE");
  }
}

async function readRecords(): Promise<StoredRSAKey[]> {
  try { return JSON.parse(await readFile(keyStorePath(), "utf8")) as StoredRSAKey[]; }
  catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return []; throw error; }
}

async function writeRecords(records: StoredRSAKey[]): Promise<void> {
  const target = keyStorePath();
  await mkdir(path.dirname(target), { recursive: true, mode: 0o700 });
  const temporary = `${target}.tmp`;
  await writeFile(temporary, JSON.stringify(records), { encoding: "utf8", mode: 0o600 });
  await rename(temporary, target);
}

function keyStorePath(): string {
  return path.join(app.getPath("userData"), "encryption", "rsa-keys.json");
}

/** privateKeyStorageDirectory 返回 safeStorage 密文文件所在目录，而不是明文私钥目录。 */
function privateKeyStorageDirectory(): string {
  return path.dirname(keyStorePath());
}
