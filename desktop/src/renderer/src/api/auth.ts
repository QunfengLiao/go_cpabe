import type { LoginData, RefreshData, UserRole, User } from "../types";
import { request } from "./request";

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
}

export function register(payload: RegisterPayload): Promise<{ user: User }> {
  return request("/auth/register", {
    method: "POST",
    skipAuth: true,
    body: JSON.stringify(payload)
  });
}

export function login(payload: LoginPayload): Promise<LoginData> {
  return request("/auth/login", {
    method: "POST",
    skipAuth: true,
    body: JSON.stringify(payload)
  });
}

export function refreshToken(refreshTokenValue: string): Promise<RefreshData> {
  return request("/auth/refresh", {
    method: "POST",
    skipAuth: true,
    skipRefresh: true,
    body: JSON.stringify({ refresh_token: refreshTokenValue })
  });
}

export function logout(refreshTokenValue: string): Promise<{ logged_out: boolean }> {
  return request("/auth/logout", {
    method: "POST",
    skipAuth: true,
    skipRefresh: true,
    body: JSON.stringify({ refresh_token: refreshTokenValue })
  });
}
