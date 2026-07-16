import type { ApiEnvelope, RefreshData } from "../types";
import {
  expireCurrentSession,
  getCurrentCachedAccount,
  saveRefreshedSession
} from "./authStorage";
import { refreshAccountSession } from "./authSessionStore";
import { getAuthRuntime, isAuthorizationReady, setAuthRuntimeFromSnapshot, waitForAuthorizationReady, waitForAuthReady, type AuthRuntimeSnapshot } from "./authRuntime";

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "http://localhost:18080/api/v1").replace(/\/+$/, "");

export class ApiError extends Error {
  status: number;
  code?: string | number;

  constructor(message: string, status: number, code?: string | number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

type RequestOptions = RequestInit & {
  skipAuth?: boolean;
  skipRefresh?: boolean;
  cacheKey?: string;
  cacheTtlMs?: number;
};

let refreshPromise: Promise<RefreshData> | null = null;
let onAuthExpired: ((message: string) => void) | null = null;
const DEFAULT_GET_CACHE_TTL_MS = 800;
const inflightGetRequests = new Map<string, Promise<unknown>>();
const responseCache = new Map<string, { expiresAt: number; value: unknown }>();

export function setAuthExpiredHandler(handler: ((message: string) => void) | null): void {
  onAuthExpired = handler;
}

export function clearRequestCache(): void {
  responseCache.clear();
  inflightGetRequests.clear();
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const cacheKey = cacheKeyFor(path, options);
  if (!cacheKey) {
    const method = String(options.method ?? "GET").toUpperCase();
    const result = await rawRequest<T>(path, options);
    if (method !== "GET") {
      // 写操作会改变列表、详情或当前用户资料，清空短缓存避免随后刷新读到旧快照。
      responseCache.clear();
    }
    return result;
  }

  const cached = responseCache.get(cacheKey);
  if (cached && cached.expiresAt > Date.now()) {
    return cached.value as T;
  }
  const inflight = inflightGetRequests.get(cacheKey);
  if (inflight) return inflight as Promise<T>;

  const promise = rawRequest<T>(path, options)
    .then((value) => {
      responseCache.set(cacheKey, { value, expiresAt: Date.now() + (options.cacheTtlMs ?? DEFAULT_GET_CACHE_TTL_MS) });
      return value;
    })
    .finally(() => {
      inflightGetRequests.delete(cacheKey);
    });
  inflightGetRequests.set(cacheKey, promise);
  return promise;
}

async function rawRequest<T>(path: string, options: RequestOptions): Promise<T> {
  const response = await fetchResponse(path, options);
  if (response.status === 401 && !options.skipRefresh) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      const retry = await fetchResponse(path, options);
      return parseResponse<T>(retry);
    }
  }
  return parseResponse<T>(response);
}

function buildURL(path: string): string {
  return `${API_BASE_URL}${path.startsWith("/") ? path : `/${path}`}`;
}

async function buildOptions(path: string, options: RequestOptions): Promise<RequestInit> {
  const headers = new Headers(options.headers);
  if (!(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const snapshot = await resolveRequestContext(path, options);
  const token = snapshot.accessToken;
  if (!options.skipAuth && token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  if (!options.skipAuth && isTenantApi(path) && snapshot.currentUserId && snapshot.currentTenantId && !headers.has("X-Tenant-Id")) {
    headers.set("X-Tenant-Id", snapshot.currentTenantId);
  }
  return {
    ...options,
    headers
  };
}

// fetchResponse 将浏览器网络层的模糊 TypeError 转成可操作提示，同时不泄漏认证头和请求正文。
async function fetchResponse(path: string, options: RequestOptions): Promise<Response> {
	try {
		return await fetch(buildURL(path), await buildOptions(path, options));
	} catch (error) {
		if (error instanceof TypeError) {
			throw new ApiError(`无法连接后端服务（${new URL(API_BASE_URL).origin}），请检查服务是否启动及 CORS 配置`, 0, "NETWORK_ERROR");
		}
		throw error;
	}
}

function cacheKeyFor(path: string, options: RequestOptions): string {
  const method = String(options.method ?? "GET").toUpperCase();
  if (method !== "GET" || options.skipAuth) return "";
  const snapshot = getAuthRuntime();
  return options.cacheKey ?? `${snapshot.currentUserId}|${snapshot.currentTenantId}|${snapshot.generation}|${snapshot.authorizationGeneration}|${method}|${buildURL(path)}`;
}

async function resolveRequestContext(path: string, options: RequestOptions): Promise<AuthRuntimeSnapshot> {
  if (options.skipAuth || isPublicApi(path)) return getAuthRuntime();
  const snapshot = isTenantApi(path)
    ? isAuthorizationContextApi(path)
      ? await waitForAuthReady()
      : await waitForAuthorizationReady()
    : getAuthRuntime();
  if (isTenantApi(path)) {
    if (!snapshot.accessToken || !snapshot.currentUserId) {
      throw new ApiError("请先登录后再访问租户资源", 401, "AUTH_REQUIRED");
    }
    if (!snapshot.currentTenantId || !snapshot.currentTenant) {
      throw new ApiError("当前账号没有可用租户上下文，请先选择租户", 403, "TENANT_CONTEXT_MISSING");
    }
    if (!isAuthorizationContextApi(path)) {
      if (snapshot.authorizationStatus === "error") {
        throw new ApiError(snapshot.authorizationError || "授权上下文加载失败", 403, "AUTHORIZATION_CONTEXT_ERROR");
      }
      if (!isAuthorizationReady(snapshot)) {
        throw new ApiError("授权上下文尚未就绪，请稍后重试", 403, "AUTHORIZATION_CONTEXT_NOT_READY");
      }
    }
  }
  return snapshot;
}

function isPublicApi(path: string): boolean {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return normalized.startsWith("/auth/");
}

function isTenantApi(path: string): boolean {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return (
    normalized.startsWith("/tenant/") ||
    /^\/tenants\/[^/]+\/(access-policy|access-policies|org-units|members|users(?:$|\/[^/]+\/attributes|\/me\/attributes))/.test(normalized)
  );
}

function isAuthorizationContextApi(path: string): boolean {
  const normalized = path.startsWith("/") ? path : `/${path}`;
  return normalized === "/tenant/me/authorization";
}

async function parseResponse<T>(response: Response): Promise<T> {
  let envelope: ApiEnvelope<T> | null = null;
  try {
    envelope = (await response.json()) as ApiEnvelope<T>;
  } catch {
    if (!response.ok) {
      throw new ApiError("后端服务不可用或响应格式错误", response.status);
    }
  }
  if (!response.ok) {
    const message = envelope?.message ?? envelope?.msg ?? "请求失败";
    throw new ApiError(message, response.status, envelope?.code);
  }
  if (!envelope) {
    throw new ApiError("后端响应为空", response.status);
  }
  return envelope.data;
}

async function tryRefresh(): Promise<boolean> {
  const account = getCurrentCachedAccount();
  if (!account) {
    expireAuth();
    return false;
  }
  try {
    refreshPromise ??= refreshAccountSession(account.userId).finally(() => {
      refreshPromise = null;
    });
    const data = await refreshPromise;
    const snapshot = saveRefreshedSession(account, data);
    setAuthRuntimeFromSnapshot(snapshot, getAuthRuntime().generation);
    return true;
  } catch {
    expireAuth();
    return false;
  }
}

function expireAuth(): void {
  expireCurrentSession();
  onAuthExpired?.("登录已过期，请重新登录");
}
