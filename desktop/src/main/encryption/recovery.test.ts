import { mkdir, readFile, utimes, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { sweepExpiredTempCiphertexts } from "./tempCiphertext";

describe("异常退出临时密文恢复", () => {
  it("只删除过期 .part，不读取或生成 DEK 恢复数据", async () => {
    const root = await import("node:fs/promises").then(({ mkdtemp }) => mkdtemp(path.join(os.tmpdir(), "gcpabe-recovery-")));
    await mkdir(root, { recursive: true });
    const expired = path.join(root, "expired.part"); const fresh = path.join(root, "fresh.part"); const unrelated = path.join(root, "keep.txt");
    await Promise.all([writeFile(expired, "cipher"), writeFile(fresh, "cipher"), writeFile(unrelated, "keep")]);
    const old = new Date(Date.now() - 48 * 60 * 60 * 1000); await utimes(expired, old, old);
    expect(await sweepExpiredTempCiphertexts(root, Date.now() - 24 * 60 * 60 * 1000)).toBe(1);
    await expect(readFile(expired)).rejects.toMatchObject({ code: "ENOENT" });
    await expect(readFile(fresh, "utf8")).resolves.toBe("cipher");
    await expect(readFile(unrelated, "utf8")).resolves.toBe("keep");
  });
});
