import path from "node:path";

// developmentWorkerCandidates 返回开发态 Worker 的受控候选路径，兼容 Electron 将 appPath 指向 desktop 或 desktop/dist 的差异。
export function developmentWorkerCandidates(appPath: string, cwd: string, executableName: string): string[] {
  const candidates = [
    path.resolve(appPath, "..", "..", "backend", "bin", executableName),
    path.resolve(appPath, "..", "backend", "bin", executableName),
    path.resolve(cwd, "..", "backend", "bin", executableName),
    path.resolve(cwd, "backend", "bin", executableName)
  ];
  return [...new Set(candidates)];
}
