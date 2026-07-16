import { app, dialog, shell } from "electron";
import { createHash, randomUUID } from "node:crypto";
import { createWriteStream } from "node:fs";
import { access, mkdir, mkdtemp, readFile, rename, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { Readable, Transform } from "node:stream";
import { pipeline } from "node:stream/promises";
import { CryptoWorkerProcess } from "./cryptoWorkerProcess";
import { validateApiBaseUrl } from "./ipcValidation";
import { loadRSAPrivateKeyForDecryption } from "./rsaKeyStore";

export interface DecryptReceivedDescriptor {
  accountId: string;
  tenantId: string;
  apiBaseUrl: string;
  accessToken: string;
  fileId: string;
  suggestedFilename: string;
}

export interface LocalDecryptionMetrics {
  ciphertext_download_duration_ms: number;
  key_recovery_duration_ms?: number;
  file_decryption_duration_ms?: number;
  plaintext_write_duration_ms?: number;
  local_decryption_duration_ms?: number;
  total_operation_duration_ms: number;
  ciphertext_size_bytes: number;
  algorithm_code: string;
  algorithm_version: string;
  success: boolean;
  failure_stage?: string;
}

export interface CiphertextSaveResult {
  cancelled: boolean;
  outputFilename?: string;
  revealToken?: string;
}

export interface PreservedCiphertextError {
  preservedCiphertextFilename: string;
  preservedCiphertextPath: string;
}

interface DecryptionMaterial {
	file_id: string;
	original_filename: string;
	plaintext_size: number;
	algorithm_code?: string;
	algorithm_version?: string;
	protected_key_format?: string;
	protected_key_base64: string;
  context_sha256: string;
  rsa_public_key_id: string;
  public_key_fingerprint_sha256: string;
  key_id?: string;
  key_envelopes?: Array<{
    key_id: string;
    protected_key_base64: string;
    context_sha256: string;
    algorithm_code: string;
    algorithm_version: string;
    protected_key_format: string;
    rsa_public_key_id: string;
    public_key_fingerprint_sha256: string;
    oaep_hash?: string;
  }>;
}

interface Envelope<T> { code: string; message: string; data: T }

export interface DecryptionCoordinatorDependencies {
  chooseDirectory: () => Promise<{ canceled: boolean; filePaths: string[] }>;
  fetchMaterial: (baseUrl: string, fileId: string, headers: Record<string, string>) => Promise<DecryptionMaterial>;
  download: (baseUrl: string, fileId: string, headers: Record<string, string>, target: string) => Promise<number>;
  loadPrivateKey: typeof loadRSAPrivateKeyForDecryption;
  createWorker: () => Pick<CryptoWorkerProcess, "decrypt" | "terminate">;
  reveal: (token: string) => Promise<boolean>;
}

const decryptedFileRevealTokens = new Map<string, string>();
let revealTokensLoaded = false;
let revealTokensLoading: Promise<void> | null = null;
let revealTokensPersisting: Promise<void> = Promise.resolve();

// decryptFile 在主进程完成授权下载、历史私钥解封和本地 Worker 调用，临时密文始终清理。
export async function decryptFile(descriptor: DecryptReceivedDescriptor, dependencies: Partial<DecryptionCoordinatorDependencies> = {}): Promise<{ cancelled: boolean; outputFilename?: string; filename?: string; revealToken?: string; metrics?: LocalDecryptionMetrics }> {
  const baseUrl = validateApiBaseUrl(descriptor.apiBaseUrl);
  const headers = { Authorization: `Bearer ${descriptor.accessToken}`, "X-Tenant-Id": descriptor.tenantId };
  const chooseDirectory = dependencies.chooseDirectory ?? (() => dialog.showOpenDialog({ title: "选择解密输出文件夹", properties: ["openDirectory", "createDirectory"] }));
  const choice = await chooseDirectory();
  if (choice.canceled || !choice.filePaths?.[0]) return { cancelled: true };
  const operationStarted = Date.now();
  const temporaryDirectory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-decrypt-"));
  const ciphertextPath = path.join(temporaryDirectory, "ciphertext.enc");
  const outputPartPath = path.join(temporaryDirectory, `.part-${randomUUID()}`);
  let targetPath = "";
  let privateKeyPEM = "";
  let worker: Pick<CryptoWorkerProcess, "decrypt" | "terminate"> | undefined;
  try {
    const downloadStarted = Date.now();
    const ciphertextSize = await (dependencies.download ?? downloadFileCiphertext)(baseUrl, descriptor.fileId, headers, ciphertextPath);
    const downloadDuration = Date.now() - downloadStarted;
    let material: DecryptionMaterial;
    try {
      material = await (dependencies.fetchMaterial ?? fetchDecryptionMaterial)(baseUrl, descriptor.fileId, headers);
    } catch (error) {
      const preserved = await preserveCiphertextPackage(ciphertextPath, choice.filePaths[0], safeFilename(`${descriptor.suggestedFilename || "decrypted-file"}.enc`));
      throw withPreservedCiphertext(error, preserved);
    }
    targetPath = await resolveNonOverwritingPath(choice.filePaths[0], safeFilename(descriptor.suggestedFilename || material.original_filename));
    const envelopes = material.key_envelopes?.length ? material.key_envelopes : [{ key_id: material.key_id ?? material.rsa_public_key_id, protected_key_base64: material.protected_key_base64, context_sha256: material.context_sha256, algorithm_code: material.algorithm_code, algorithm_version: material.algorithm_version, protected_key_format: material.protected_key_format, rsa_public_key_id: material.rsa_public_key_id, public_key_fingerprint_sha256: material.public_key_fingerprint_sha256 }];
    let decrypted: Awaited<ReturnType<CryptoWorkerProcess["decrypt"]>> | undefined;
    let lastDecryptError: unknown;
    for (const envelope of envelopes) {
      try {
        privateKeyPEM = await (dependencies.loadPrivateKey ?? loadRSAPrivateKeyForDecryption)(descriptor.accountId, descriptor.tenantId, envelope.rsa_public_key_id, envelope.public_key_fingerprint_sha256);
      } catch (error) {
        lastDecryptError = error;
        continue;
      }
      try {
        worker ??= (dependencies.createWorker ?? (() => new CryptoWorkerProcess()))();
        decrypted = await worker.decrypt({ ciphertext_path: ciphertextPath, output_path: outputPartPath, private_key_pem: privateKeyPEM, protected_key_base64: envelope.protected_key_base64, context_sha256: envelope.context_sha256, tenant_id: Number(descriptor.tenantId), file_id: material.file_id, rsa_public_key_id: envelope.rsa_public_key_id }, () => undefined);
        break;
      } catch (error) {
        lastDecryptError = error;
        await rm(outputPartPath, { force: true });
      } finally {
        privateKeyPEM = "";
      }
    }
    if (!decrypted) {
      // 密文本身仍是用户可保留的原始证据；失败不应把它随临时目录一起静默删除。
      const preserved = await preserveCiphertextPackage(ciphertextPath, choice.filePaths[0], safeFilename(`${descriptor.suggestedFilename || material.original_filename}.enc`));
      throw withPreservedCiphertext(lastDecryptError ?? new Error("LOCAL_RSA_PRIVATE_KEY_NOT_FOUND"), preserved);
    }
    const renameStarted = Date.now();
    await rename(outputPartPath, targetPath);
    const plaintextWriteDuration = (decrypted.plaintextWriteMs ?? 0) + (Date.now() - renameStarted);
    const revealToken = rememberDecryptedFile(targetPath);
    await (dependencies.reveal ?? revealDecryptedFile)(revealToken);
    return {
      cancelled: false,
      outputFilename: path.basename(targetPath),
      filename: path.basename(targetPath),
      revealToken,
      metrics: {
        ciphertext_download_duration_ms: downloadDuration,
        key_recovery_duration_ms: decrypted.keyRecoveryMs,
        file_decryption_duration_ms: decrypted.fileDecryptionMs,
        plaintext_write_duration_ms: plaintextWriteDuration,
        local_decryption_duration_ms: sumDefined(decrypted.keyRecoveryMs, decrypted.fileDecryptionMs) ?? decrypted.decryptMs,
        total_operation_duration_ms: Date.now() - operationStarted,
        ciphertext_size_bytes: ciphertextSize,
        algorithm_code: material.algorithm_code || "RSA-OAEP-SHA256",
        algorithm_version: material.algorithm_version || "1",
        success: true
      }
    };
  } finally {
    privateKeyPEM = "";
    worker?.terminate();
    await rm(outputPartPath, { force: true });
    await rm(temporaryDirectory, { recursive: true, force: true });
  }
}

// saveCiphertext 通过主进程流式保存原始密文，供用户不需要立即恢复明文时保留 .enc 文件。
export async function saveCiphertext(descriptor: DecryptReceivedDescriptor): Promise<CiphertextSaveResult> {
  const baseUrl = validateApiBaseUrl(descriptor.apiBaseUrl);
  const headers = { Authorization: `Bearer ${descriptor.accessToken}`, "X-Tenant-Id": descriptor.tenantId };
  const choice = await dialog.showOpenDialog({ title: "选择密文保存文件夹", properties: ["openDirectory", "createDirectory"] });
  if (choice.canceled || !choice.filePaths?.[0]) return { cancelled: true };
  const temporaryDirectory = await mkdtemp(path.join(os.tmpdir(), "go-cpabe-ciphertext-"));
  const temporaryPath = path.join(temporaryDirectory, "ciphertext.enc");
  try {
    await downloadFileCiphertext(baseUrl, descriptor.fileId, headers, temporaryPath);
    const targetPath = await resolveNonOverwritingPath(choice.filePaths[0], safeFilename(`${descriptor.suggestedFilename || "encrypted-file"}.enc`));
    await rename(temporaryPath, targetPath);
    const revealToken = rememberDecryptedFile(targetPath);
    return { cancelled: false, outputFilename: path.basename(targetPath), revealToken };
  } finally {
    await rm(temporaryPath, { force: true });
    await rm(temporaryDirectory, { recursive: true, force: true });
  }
}

// decryptReceivedFile 保留旧 IPC 的兼容名称，所有行为统一委托给 decryptFile。
export async function decryptReceivedFile(descriptor: DecryptReceivedDescriptor): ReturnType<typeof decryptFile> {
  return decryptFile(descriptor);
}

// rememberDecryptedFile 只把真实明文路径保存在主进程内存中，渲染层只能拿到不可反推出路径的 token。
export function rememberDecryptedFile(targetPath: string): string {
  const revealToken = randomUUID();
  decryptedFileRevealTokens.set(revealToken, targetPath);
  void ensureRevealTokensLoaded().then(() => schedulePersistRevealTokens()).catch((error) => console.error("本地下载记录保存失败", error));
  return revealToken;
}

// revealDecryptedFile 通过主进程 token 打开上次解密文件所在位置；路径失效时清理 token，避免回退成重复解密。
export async function revealDecryptedFile(revealToken: string, reveal?: (targetPath: string) => void | Promise<void>): Promise<boolean> {
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(revealToken)) return false;
  await ensureRevealTokensLoaded();
  const targetPath = decryptedFileRevealTokens.get(revealToken);
  if (!targetPath) return false;
  try {
    await access(targetPath);
  } catch {
    return false;
  }
  if (reveal) {
    await reveal(targetPath);
  } else {
    const openError = await shell.openPath(path.dirname(targetPath));
    if (openError) return false;
  }
  return true;
}

// clearDecryptionRevealTokens 在账号、租户或运行时上下文切换时清空本地明文路径映射，避免旧账号继续引用上一会话位置。
export function clearDecryptionRevealTokens(): void {
  decryptedFileRevealTokens.clear();
  revealTokensLoaded = false;
}

/** ensureRevealTokensLoaded 恢复主进程重启前的本地文件定位令牌；只读写主进程私有元数据。 */
async function ensureRevealTokensLoaded(): Promise<void> {
  if (revealTokensLoaded) return;
  if (!revealTokensLoading) {
    revealTokensLoading = readFile(revealTokenStorePath(), "utf8")
      .then((content) => {
        const parsed = JSON.parse(content) as unknown;
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
          for (const [token, targetPath] of Object.entries(parsed)) {
            if (isRevealToken(token) && typeof targetPath === "string" && targetPath.length > 0) decryptedFileRevealTokens.set(token, targetPath);
          }
        }
      })
      .catch((error: unknown) => {
        if ((error as NodeJS.ErrnoException).code !== "ENOENT") return;
      })
      .finally(() => {
        revealTokensLoaded = true;
        revealTokensLoading = null;
      });
  }
  await revealTokensLoading;
}

/** persistRevealTokens 将文件定位令牌限制在主进程目录，Renderer 永远只能保存不可反推出路径的 UUID。 */
async function persistRevealTokens(): Promise<void> {
  const target = revealTokenStorePath();
  await mkdir(path.dirname(target), { recursive: true, mode: 0o700 });
  const temporary = `${target}.tmp`;
  await writeFile(temporary, JSON.stringify(Object.fromEntries(decryptedFileRevealTokens)), { encoding: "utf8", mode: 0o600 });
  await rename(temporary, target);
}

/** schedulePersistRevealTokens 串行化本地下载记录写入，避免连续下载时多个临时文件互相覆盖。 */
function schedulePersistRevealTokens(): Promise<void> {
  revealTokensPersisting = revealTokensPersisting.catch(() => undefined).then(() => persistRevealTokens());
  return revealTokensPersisting;
}

/** revealTokenStorePath 返回本地下载记录的主进程私有存储位置。 */
function revealTokenStorePath(): string {
  const userData = typeof app?.getPath === "function" ? app.getPath("userData") : os.tmpdir();
  return path.join(userData, "encryption", "decryption-reveal-tokens.json");
}

/** isRevealToken 过滤恢复文件中的任意非令牌字段，避免污染内存映射。 */
function isRevealToken(value: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

// fetchJSON 读取受认证的解密材料响应，错误只返回服务端业务消息而不包含认证头。
async function fetchJSON<T>(url: string, headers: Record<string, string>): Promise<T> {
  const response = await fetch(url, { headers });
  const payload = await response.json() as Envelope<T>;
  if (!response.ok || payload.code !== "OK") throw new Error(payload.message || "获取解密材料失败");
  return payload.data;
}

async function fetchDecryptionMaterial(baseUrl: string, fileId: string, headers: Record<string, string>): Promise<DecryptionMaterial> {
  return fetchJSON<DecryptionMaterial>(`${baseUrl}/api/v1/tenant/files/${fileId}/decryption-material`, headers);
}

async function downloadFileCiphertext(baseUrl: string, fileId: string, headers: Record<string, string>, target: string): Promise<number> {
  return downloadCiphertext(`${baseUrl}/api/v1/tenant/files/${fileId}/ciphertext`, headers, target);
}

// preserveCiphertextPackage 在本地解密无法完成时，把已下载的原始密文移出临时目录，避免用户丢失可再次处理的密文包。
async function preserveCiphertextPackage(source: string, directory: string, filename: string): Promise<string> {
  const target = await resolveNonOverwritingPath(directory, filename);
  await rename(source, target);
  return target;
}

// withPreservedCiphertext 把密文保留事实附加到主进程错误对象；绝对路径只留在主进程，Renderer 只接收文件名和目录 token。
function withPreservedCiphertext(error: unknown, preservedPath: string): Error & PreservedCiphertextError {
  const result = error instanceof Error ? error : new Error(String(error));
  return Object.assign(result, { preservedCiphertextFilename: path.basename(preservedPath), preservedCiphertextPath: preservedPath });
}

// downloadCiphertext 流式保存密文并核对服务端 SHA-256，摘要不一致时拒绝进入私钥解封阶段。
async function downloadCiphertext(url: string, headers: Record<string, string>, target: string): Promise<number> {
  const response = await fetch(url, { headers });
  if (!response.ok || !response.body) throw new Error("密文下载失败");
  const expected = response.headers.get("X-Ciphertext-SHA256")?.toLowerCase();
  if (!expected || !/^[0-9a-f]{64}$/.test(expected)) throw new Error("密文摘要缺失");
  const hasher = createHash("sha256");
  const digesting = new Transform({ transform(chunk, _encoding, callback) { hasher.update(chunk); callback(null, chunk); } });
  let downloadedBytes = 0;
  const counting = new Transform({ transform(chunk: Buffer, _encoding, callback) { downloadedBytes += chunk.length; callback(null, chunk); } });
  await pipeline(Readable.fromWeb(response.body as never), digesting, counting, createWriteStream(target, { mode: 0o600, flags: "wx" }));
  if (hasher.digest("hex") !== expected) throw new Error("密文完整性校验失败");
  return downloadedBytes;
}

// safeFilename 去除路径分隔符和 Windows 保留字符，避免服务端文件名影响本地保存目录。
export function safeFilename(value: string): string {
  const cleaned = path.basename(value).replace(/[<>:"/\\|?*\u0000-\u001f]/g, "_").trim();
  return cleaned || "decrypted-file";
}

// resolveNonOverwritingPath 在用户选择的目录内生成不覆盖已有文件的目标路径。
export async function resolveNonOverwritingPath(directory: string, filename: string): Promise<string> {
  const parsed = path.parse(safeFilename(filename));
  const base = parsed.name || "decrypted-file";
  const ext = parsed.ext;
  for (let index = 0; index < 10000; index++) {
    const suffix = index === 0 ? "" : ` (${index})`;
    const candidate = path.join(directory, `${base}${suffix}${ext}`);
    try {
      await access(candidate);
    } catch {
      return candidate;
    }
  }
  throw new Error("DECRYPTION_OUTPUT_NAME_EXHAUSTED");
}

function sumDefined(left?: number, right?: number): number | undefined {
  if (left == null && right == null) return undefined;
  return (left ?? 0) + (right ?? 0);
}
