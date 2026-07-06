import type { User } from "../types";
import { request } from "./request";

export interface UpdateProfilePayload {
  nickname: string;
  bio: string;
  birthday: string;
}

export function getCurrentUser(): Promise<{ user: User }> {
  return request("/users/me");
}

export function updateCurrentUser(payload: UpdateProfilePayload): Promise<{ user: User }> {
  return request("/users/me", {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export function uploadAvatar(file: File): Promise<{ avatar_url: string }> {
  const form = new FormData();
  form.append("avatar", file);
  return request("/users/me/avatar", {
    method: "POST",
    body: form
  });
}
