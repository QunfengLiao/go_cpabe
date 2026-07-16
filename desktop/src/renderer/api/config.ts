const fallbackBaseUrl = "http://localhost:18080";
declare const __APP_ENV__: string;
declare const __APP_VERSION__: string;

export function getApiBaseUrl(): string {
  const fromDesktopEnv = import.meta.env.DESKTOP_API_BASE_URL;
  const fromViteEnv = import.meta.env.VITE_DESKTOP_API_BASE_URL;
  return normalizeBaseUrl(fromDesktopEnv || fromViteEnv || fallbackBaseUrl);
}

export function getAppEnv(): string {
  return __APP_ENV__ || "development";
}

export function getAppVersion(): string {
  return `v${__APP_VERSION__ || "0.1.0"}`;
}

function normalizeBaseUrl(value: string): string {
  return value.replace(/\/+$/, "");
}
