import type { ApiEnvelope, RefreshData } from "../types";
import {
  expireCurrentSession,
  getCurrentCachedAccount,
  getRefreshToken,
  saveRefreshedSession,
  saveTokens,
  getAccessToken
} from "./authStorage";

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
};

let refreshPromise: Promise<RefreshData> | null = null;
let onAuthExpired: ((message: string) => void) | null = null;

export function setAuthExpiredHandler(handler: ((message: string) => void) | null): void {
  onAuthExpired = handler;
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await rawRequest<T>(path, options);
  return response;
}

async function rawRequest<T>(path: string, options: RequestOptions): Promise<T> {
  const response = await fetch(buildURL(path), buildOptions(options));
  if (response.status === 401 && !options.skipRefresh) {
    const refreshed = await tryRefresh();
    if (refreshed) {
      const retry = await fetch(buildURL(path), buildOptions(options));
      return parseResponse<T>(retry);
    }
  }
  return parseResponse<T>(response);
}

function buildURL(path: string): string {
  return `${API_BASE_URL}${path.startsWith("/") ? path : `/${path}`}`;
}

function buildOptions(options: RequestOptions): RequestInit {
  const headers = new Headers(options.headers);
  if (!(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const token = getAccessToken();
  if (!options.skipAuth && token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return {
    ...options,
    headers
  };
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
  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    expireAuth();
    return false;
  }
  try {
    refreshPromise ??= refreshTokenRequest(refreshToken).finally(() => {
      refreshPromise = null;
    });
    const data = await refreshPromise;
    const account = getCurrentCachedAccount();
    if (account) {
      saveRefreshedSession(account, data);
    } else {
      saveTokens(data.access_token, data.refresh_token ?? refreshToken);
    }
    return true;
  } catch {
    expireAuth();
    return false;
  }
}

async function refreshTokenRequest(refreshToken: string): Promise<RefreshData> {
  const response = await fetch(buildURL("/auth/refresh"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken })
  });
  return parseResponse<RefreshData>(response);
}

function expireAuth(): void {
  expireCurrentSession();
  onAuthExpired?.("登录已过期，请重新登录");
}
