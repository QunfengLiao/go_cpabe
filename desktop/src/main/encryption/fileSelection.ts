import { dialog } from "electron";
import { randomUUID } from "node:crypto";
import { stat } from "node:fs/promises";
import path from "node:path";
import type { SelectedFileHandle } from "./types";

interface StoredHandle extends SelectedFileHandle { absolutePath: string }

const handles = new Map<string, StoredHandle>();

export async function selectSingleFile(): Promise<SelectedFileHandle | null> {
  const result = await dialog.showOpenDialog({ properties: ["openFile"], title: "选择要加密的文件" });
  if (result.canceled || result.filePaths.length !== 1) return null;
  const absolutePath = path.resolve(result.filePaths[0]);
  const info = await stat(absolutePath);
  if (!info.isFile() || info.size <= 0) throw new Error("FILE_INVALID");
  const handle: StoredHandle = { handleId: randomUUID(), absolutePath, name: path.basename(absolutePath), size: info.size, displayMimeType: "application/octet-stream", lastModifiedMs: info.mtimeMs };
  handles.set(handle.handleId, handle);
  return publicHandle(handle);
}

export function resolveFileHandle(handleId: string): StoredHandle {
  const handle = handles.get(handleId);
  if (!handle) throw new Error("FILE_HANDLE_EXPIRED");
  return handle;
}

export function releaseFileHandle(handleId: string): void {
  handles.delete(handleId);
}

export function releaseAllFileHandles(): void {
  handles.clear();
}

function publicHandle(handle: StoredHandle): SelectedFileHandle {
  const { absolutePath: _hidden, ...safe } = handle;
  return safe;
}
