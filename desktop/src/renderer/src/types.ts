export type UserRole = "admin" | "data_owner" | "data_user";
export type UserStatus = "active" | "disabled";

export interface User {
  id: number;
  email: string;
  nickname: string;
  role: UserRole;
  avatar_url: string;
  bio: string;
  birthday: string | null;
  status?: UserStatus;
  created_at: string;
  updated_at?: string;
}

export interface ApiEnvelope<T> {
  code: string | number;
  message?: string;
  msg?: string;
  data: T;
  request_id?: string;
}

export interface LoginData {
  access_token: string;
  access_token_expires_in: number;
  refresh_token: string;
  refresh_token_expires_in: number;
  token_type: string;
  user: User;
}

export interface RefreshData {
  access_token: string;
  access_token_expires_in: number;
  refresh_token?: string;
  refresh_token_expires_in?: number;
  token_type: string;
}

export interface CachedAccount {
  userId: string;
  email: string;
  nickname: string;
  role: UserRole;
  avatarUrl?: string;
  user?: User;
  refreshToken: string;
  refreshTokenExpiresAt?: number;
  lastActiveAt: number;
  expired?: boolean;
  loggedOut?: boolean;
}

export interface AuthStateSnapshot {
  currentUserId: string;
  accessToken: string;
  refreshToken: string;
  user: User | null;
  cachedAccounts: CachedAccount[];
}
