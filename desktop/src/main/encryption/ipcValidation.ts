import type { IpcMainInvokeEvent } from "electron";
import path from "node:path";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function validateIpcSender(event: IpcMainInvokeEvent): void {
	const frame = event.senderFrame;
	if (!frame || frame !== event.sender.mainFrame) throw new Error("IPC_SENDER_REJECTED");
	const url = new URL(frame.url);
  if (url.protocol === "file:") return;
  const allowed = process.env.ELECTRON_RENDERER_URL;
	if (process.env.NODE_ENV !== "production" && allowed && frame.url.startsWith(allowed)) return;
  throw new Error("IPC_SENDER_REJECTED");
}

export function requireUUID(value: string, field: string): string {
  if (!UUID_PATTERN.test(value)) throw new Error(`${field.toUpperCase()}_INVALID`);
  return value;
}

export function requireTenantId(value: string): string {
  if (!/^[1-9][0-9]*$/.test(value)) throw new Error("TENANT_ID_INVALID");
  return value;
}

export function requireOpaqueFileHandle(value: string): string {
  return requireUUID(value, "file_handle");
}

export function requireAlgorithm(code: string, version: string): void {
  if (code !== "RSA-OAEP-SHA256" || version !== "1") throw new Error("ALGORITHM_UNAVAILABLE");
}

export function validateApiBaseUrl(value: string): string {
  const url = new URL(value);
  const configured = (process.env.ENCRYPTION_API_ALLOWLIST ?? "http://127.0.0.1:18080,http://localhost:18080").split(",").map((item) => item.trim()).filter(Boolean);
  const normalized = `${url.protocol}//${url.host}`;
  if (!configured.includes(normalized) || !["http:", "https:"].includes(url.protocol)) throw new Error("API_BASE_URL_REJECTED");
  return normalized;
}

export function assertControlledPath(candidate: string, root: string): string {
  const resolvedRoot = path.resolve(root);
  const resolved = path.resolve(candidate);
  if (resolved !== resolvedRoot && !resolved.startsWith(`${resolvedRoot}${path.sep}`)) throw new Error("PATH_REJECTED");
  return resolved;
}
