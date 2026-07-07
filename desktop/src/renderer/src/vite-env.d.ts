/// <reference types="vite/client" />

interface DesktopCredentialStore {
  saveCredential(email: string, password: string): Promise<void>;
  getSavedEmails(): Promise<string[]>;
  getCredentialByEmail(email: string): Promise<string>;
  removeCredential(email: string): Promise<void>;
}

interface Window {
  desktopRuntime?: {
    platform: string;
  };
  desktopCredentialStore?: DesktopCredentialStore;
}
