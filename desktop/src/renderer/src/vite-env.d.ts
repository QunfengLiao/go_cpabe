/// <reference types="vite/client" />

interface DesktopCredentialStore {
  saveCredential(email: string, password: string): Promise<void>;
  getSavedEmails(): Promise<string[]>;
  getCredentialByEmail(email: string): Promise<string>;
  removeCredential(email: string): Promise<void>;
}

interface DesktopAuthSessionStore {
  getDeviceId(): Promise<string>;
  saveSession(accountId: string, refreshToken: string, expiresAt?: number): Promise<void>;
  hasSession(accountId: string): Promise<boolean>;
  refreshSession(accountId: string): Promise<import("./types").RefreshData>;
  logoutSession(accountId: string): Promise<void>;
  removeSession(accountId: string): Promise<void>;
}

interface Window {
  desktopRuntime?: {
    platform: string;
  };
  desktopCredentialStore?: DesktopCredentialStore;
  desktopAuthSessionStore?: DesktopAuthSessionStore;
}
