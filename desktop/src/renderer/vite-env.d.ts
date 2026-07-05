/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly DESKTOP_API_BASE_URL?: string;
  readonly VITE_DESKTOP_API_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

declare const __APP_ENV__: string;
declare const __APP_VERSION__: string;
