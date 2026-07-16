import { randomUUID } from "node:crypto";
import { mkdir, open, readdir, rm, stat } from "node:fs/promises";
import path from "node:path";

export async function createTempCiphertextPath(root: string): Promise<string> {
  await mkdir(root, { recursive: true, mode: 0o700 });
  const candidate = path.join(root, `${randomUUID()}.part`);
  const file = await open(candidate, "wx", 0o600);
  await file.close();
  await rm(candidate, { force: true });
  return candidate;
}

export async function removeTempCiphertext(candidate: string): Promise<void> {
  await rm(candidate, { force: true });
}

export async function sweepExpiredTempCiphertexts(root: string, cutoffMs: number): Promise<number> {
  await mkdir(root, { recursive: true, mode: 0o700 });
  const names = await readdir(root);
  let deleted = 0;
  for (const name of names) {
    if (!name.endsWith(".part")) continue;
    const candidate = path.join(root, name);
    const info = await stat(candidate);
    if (!info.isFile() || info.mtimeMs > cutoffMs) continue;
    await rm(candidate, { force: true });
    deleted += 1;
  }
  return deleted;
}
