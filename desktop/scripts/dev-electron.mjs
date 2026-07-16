import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import path from "node:path";
import electronExecutable from "electron";

const projectRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const rendererUrl = "http://127.0.0.1:5173";
const nodeExecutable = process.execPath;
const tscCli = path.join(projectRoot, "node_modules", "typescript", "bin", "tsc");
const viteCli = path.join(projectRoot, "node_modules", "vite", "bin", "vite.js");

const childProcesses = new Set();
let electronProcess;
let isShuttingDown = false;
let mainBuildReady = false;
let restartTimer;

// startChild 统一继承当前终端与租户环境，确保各 dev:* 命令的行为保持一致。
function startChild(command, args, options = {}) {
  const child = spawn(command, args, {
    cwd: projectRoot,
    env: process.env,
    stdio: "inherit",
    ...options
  });

  childProcesses.add(child);
  child.once("exit", () => childProcesses.delete(child));
  return child;
}

// stopChild 只终止本脚本创建的直接子进程，避免影响其他正在运行的 Node 服务。
function stopChild(child) {
  if (child && child.exitCode === null && !child.killed) {
    child.kill();
  }
}

// shutdown 在用户关闭窗口或中断命令时清理监听进程，防止残留 Vite 端口。
function shutdown(exitCode = 0) {
  if (isShuttingDown) return;
  isShuttingDown = true;
  clearTimeout(restartTimer);

  for (const child of childProcesses) {
    stopChild(child);
  }

  setTimeout(() => process.exit(exitCode), 100).unref();
}

// startElectron 在渲染服务与首次主进程编译都就绪后启动桌面窗口。
function startElectron() {
  const child = startChild(electronExecutable, ["dist/main/main.js"], {
    env: {
      ...process.env,
      ELECTRON_RENDERER_URL: rendererUrl
    }
  });

  electronProcess = child;
  child.once("exit", (code) => {
    if (!isShuttingDown && electronProcess === child) {
      shutdown(code ?? 0);
    }
  });
}

// restartElectron 让主进程和 preload 修改也能生效；渲染进程修改由 Vite HMR 直接更新。
function restartElectron() {
  clearTimeout(restartTimer);
  restartTimer = setTimeout(() => {
    const previousProcess = electronProcess;
    electronProcess = undefined;
    stopChild(previousProcess);
    startElectron();
  }, 150);
}

// waitForRenderer 轮询本地服务，避免 Electron 在 Vite 尚未监听端口时显示加载失败。
async function waitForRenderer(timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    try {
      const response = await fetch(rendererUrl);
      if (response.ok) return;
    } catch {
      // Vite 启动期间连接失败属于预期状态，短暂等待后重试。
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }

  throw new Error(`Vite 开发服务器启动超时：${rendererUrl}`);
}

// startTypeScriptWatcher 根据成功编译提示启动或重启 Electron，编译失败时保留当前窗口便于排错。
function startTypeScriptWatcher() {
  let outputBuffer = "";
  let resolveFirstBuild;
  const firstBuild = new Promise((resolve) => {
    resolveFirstBuild = resolve;
  });

  const watcher = startChild(
    nodeExecutable,
    [tscCli, "-p", "tsconfig.electron.json", "--watch", "--preserveWatchOutput"],
    { stdio: ["inherit", "pipe", "inherit"] }
  );

  watcher.stdout.on("data", (chunk) => {
    const text = chunk.toString();
    process.stdout.write(text);
    outputBuffer = `${outputBuffer}${text}`.slice(-2000);

    if (/Found 0 errors?\./.test(outputBuffer)) {
      outputBuffer = "";
      if (!mainBuildReady) {
        mainBuildReady = true;
        resolveFirstBuild();
      } else if (electronProcess) {
        restartElectron();
      }
    }
  });

  watcher.once("exit", (code) => {
    if (!isShuttingDown) shutdown(code ?? 1);
  });

  return firstBuild;
}

process.once("SIGINT", () => shutdown(0));
process.once("SIGTERM", () => shutdown(0));

const firstMainBuild = startTypeScriptWatcher();
const viteProcess = startChild(
  nodeExecutable,
  [viteCli, "--host", "127.0.0.1", "--port", "5173", "--strictPort"]
);

viteProcess.once("exit", (code) => {
  if (!isShuttingDown) shutdown(code ?? 1);
});

try {
  await Promise.all([firstMainBuild, waitForRenderer()]);
  startElectron();
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  shutdown(1);
}
