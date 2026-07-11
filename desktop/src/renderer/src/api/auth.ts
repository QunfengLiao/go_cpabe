import type { LoginData, UserRole, User } from "../types";
import { request } from "./request";
import { getDeviceId } from "./authSessionStore";

export interface RegisterPayload {
  email: string;
  password: string;
  confirm_password: string;
  nickname: string;
  role: Exclude<UserRole, "admin">;
}

export interface LoginPayload {
  email: string;
  password: string;
  tenantCode?: string;
}

export function register(payload: RegisterPayload): Promise<{ user: User }> {
  return request("/auth/register", {
    method: "POST",
    skipAuth: true,
    body: JSON.stringify(payload)
  });
}

export async function login(payload: LoginPayload): Promise<LoginData> {
  return request("/auth/login", {
    method: "POST",
    skipAuth: true,
    body: JSON.stringify({ ...payload, device_id: await getDeviceId() })
  });
}
