import { app } from "electron";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";
import { sweepExpiredTempCiphertexts } from "./tempCiphertext";
import { EncryptionTaskApiClient } from "./taskApiClient";

export interface InterruptedExecution {
  executionId: string;
  accountId: string;
  tenantId: string;
  taskId: string;
  attemptId: string;
  updatedAt: string;
}

export interface RecoverySummary { expiredTempFiles: number; interruptedExecutions: InterruptedExecution[] }

export async function recordActiveExecution(record: InterruptedExecution): Promise<void> {
  const records = await readExecutionIndex();
  const filtered = records.filter((item) => item.executionId !== record.executionId);
  filtered.push(record);
  await writeExecutionIndex(filtered);
}

export async function removeActiveExecution(executionId: string): Promise<void> {
  const records = await readExecutionIndex();
  await writeExecutionIndex(records.filter((item) => item.executionId !== executionId));
}

export async function runEncryptionRecovery(): Promise<RecoverySummary> {
  const root = path.join(app.getPath("temp"), "go-cpabe", "ciphertexts");
  const expiredTempFiles = await sweepExpiredTempCiphertexts(root, Date.now() - 24 * 60 * 60 * 1000);
  return { expiredTempFiles, interruptedExecutions: await readExecutionIndex() };
}

export async function reportInterruptedExecutions(accountId: string, tenantId: string, apiBaseUrl: string, accessToken: string): Promise<number> {
  const records = await readExecutionIndex(); const client = new EncryptionTaskApiClient(apiBaseUrl, accessToken, tenantId); let reported = 0;
  for (const record of records) {
    if (record.accountId !== accountId || record.tenantId !== tenantId) continue;
    try { await client.fail(record.taskId, record.attemptId, "CLIENT_INTERRUPTED", true); await removeActiveExecution(record.executionId); reported += 1; } catch { /* 保留非敏感索引，等待同账号同租户下次重试上报。 */ }
  }
  return reported;
}

async function readExecutionIndex(): Promise<InterruptedExecution[]> {
  try {
    const parsed = JSON.parse(await readFile(executionIndexPath(), "utf8")) as InterruptedExecution[];
    return parsed.filter((item) => item.executionId && item.accountId && item.tenantId && item.taskId && item.attemptId);
  } catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return []; throw error; }
}

async function writeExecutionIndex(records: InterruptedExecution[]): Promise<void> {
  const target = executionIndexPath(); await mkdir(path.dirname(target), { recursive: true, mode: 0o700 }); const temporary = `${target}.tmp`;
  await writeFile(temporary, JSON.stringify(records), { encoding: "utf8", mode: 0o600 }); await rename(temporary, target);
}

function executionIndexPath(): string { return path.join(app.getPath("userData"), "encryption", "active-executions.json"); }
