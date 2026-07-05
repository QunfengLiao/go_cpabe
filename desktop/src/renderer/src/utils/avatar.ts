import { API_BASE_URL } from "../api/request";

export function resolveAvatarURL(value?: string | null): string {
  if (!value) return "";
  if (/^https?:\/\//.test(value)) return value;
  const base = API_BASE_URL.replace(/\/api\/v1$/, "");
  return `${base}${value.startsWith("/") ? value : `/${value}`}`;
}

export function avatarInitial(nickname?: string | null, email?: string | null): string {
  return (nickname?.trim().slice(0, 1) || email?.trim().slice(0, 1) || "U").toUpperCase();
}
