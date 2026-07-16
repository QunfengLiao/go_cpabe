import path from "node:path";
import { describe, expect, it } from "vitest";
import { developmentWorkerCandidates } from "./cryptoWorkerPath";

describe("开发态 Crypto Worker 路径", () => {
  it("appPath 指向 desktop/dist 时回到仓库 backend/bin", () => {
    const repository = path.resolve("D:/workspace/go_cpabe");
    const candidates = developmentWorkerCandidates(path.join(repository, "desktop", "dist"), path.join(repository, "desktop"), "crypto-worker.exe");
    expect(candidates[0]).toBe(path.join(repository, "backend", "bin", "crypto-worker.exe"));
  });

  it("appPath 指向 desktop 时仍包含仓库 backend/bin", () => {
    const repository = path.resolve("D:/workspace/go_cpabe");
    const expected = path.join(repository, "backend", "bin", "crypto-worker.exe");
    const candidates = developmentWorkerCandidates(path.join(repository, "desktop"), path.join(repository, "desktop"), "crypto-worker.exe");
    expect(candidates).toContain(expected);
    expect(new Set(candidates).size).toBe(candidates.length);
  });
});
