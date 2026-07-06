import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { logout as logoutRequest, refreshToken as refreshTokenRequest } from "../api/auth";
import {
  clearCurrentSession,
  expireCurrentSession,
  getCachedAccounts,
  getAuthSnapshot,
  getRefreshToken,
  markCachedAccountExpired,
  markCachedAccountLoggedOut,
  removeCachedAccount,
  saveLoginSession,
  saveRefreshedSession,
  saveTokens,
  saveUser
} from "../api/authStorage";
import { getCurrentUser } from "../api/user";
import { ApiError } from "../api/request";
import { setAuthExpiredHandler } from "../api/request";
import type { CachedAccount, LoginData, User } from "../types";

interface AuthContextValue {
  currentUserId: string;
  accessToken: string;
  refreshToken: string;
  user: User | null;
  cachedAccounts: CachedAccount[];
  isAuthenticated: boolean;
  notice: string;
  setTokens: (accessToken: string, refreshToken: string) => void;
  setUser: (user: User | null) => void;
  finishLogin: (data: LoginData) => void;
  switchAccount: (accountId: string) => Promise<void>;
  removeAccount: (accountId: string) => Promise<void>;
  clearAuth: (message?: string) => void;
  logout: () => Promise<void>;
  consumeNotice: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();
  const initial = getAuthSnapshot();
  const [currentUserId, setCurrentUserId] = useState(initial.currentUserId);
  const [accessToken, setAccessToken] = useState(initial.accessToken);
  const [refreshToken, setRefreshToken] = useState(initial.refreshToken);
  const [user, setUserState] = useState<User | null>(initial.user);
  const [cachedAccounts, setCachedAccounts] = useState<CachedAccount[]>(initial.cachedAccounts);
  const [notice, setNotice] = useState("");

  function syncSnapshot() {
    const snapshot = getAuthSnapshot();
    setCurrentUserId(snapshot.currentUserId);
    setAccessToken(snapshot.accessToken);
    setRefreshToken(snapshot.refreshToken);
    setUserState(snapshot.user);
    setCachedAccounts(snapshot.cachedAccounts);
  }

  const setTokens = useCallback((nextAccessToken: string, nextRefreshToken: string) => {
    saveTokens(nextAccessToken, nextRefreshToken);
    setAccessToken(nextAccessToken);
    setRefreshToken(nextRefreshToken);
  }, []);

  const setUser = useCallback((nextUser: User | null) => {
    saveUser(nextUser);
    setUserState(nextUser);
    setCachedAccounts(getCachedAccounts());
  }, []);

  const clearAuth = useCallback((message?: string) => {
    clearCurrentSession();
    syncSnapshot();
    if (message) setNotice(message);
  }, []);

  const finishLogin = useCallback((data: LoginData) => {
    saveLoginSession(data);
    syncSnapshot();
  }, []);

  const switchAccount = useCallback(
    async (accountId: string) => {
      const account = getCachedAccounts().find((item) => item.userId === accountId);
      if (!account || !account.refreshToken || account.expired || account.loggedOut) {
        if (account) {
          setCachedAccounts(markCachedAccountExpired(account.userId));
        }
        throw new ApiError("该账号登录已过期，请重新登录", 401);
      }

      try {
        const data = await refreshTokenRequest(account.refreshToken);
        // refresh_token 权限较高，当前仅为开发阶段把多个账号的 refresh_token 放在 localStorage，
        // 生产环境更推荐 HttpOnly Cookie、桌面端安全存储或系统 Keychain；这里也绝不展示或打印 token。
        saveRefreshedSession(account, data, account.user ?? null);
        syncSnapshot();

        try {
          const current = await getCurrentUser();
          saveUser(current.user);
          syncSnapshot();
        } catch {
          // 切换成功以 refresh 结果为准；资料刷新失败不回滚账号，资料页后续仍会重新拉取。
        }

        navigate("/profile", { replace: true });
      } catch (err) {
        setCachedAccounts(markCachedAccountExpired(account.userId));
        throw err instanceof ApiError ? err : new ApiError("该账号登录已过期，请重新登录", 401);
      }
    },
    [navigate]
  );

  const logout = useCallback(async () => {
    const token = getRefreshToken();
    const logoutUserId = currentUserId;
    try {
      if (token) {
        await logoutRequest(token);
      }
    } finally {
      if (logoutUserId) {
        markCachedAccountLoggedOut(logoutUserId);
      }
      clearCurrentSession();
      syncSnapshot();
      const hasSwitchableAccount = getCachedAccounts().some((account) => account.refreshToken && !account.expired && !account.loggedOut);
      navigate(hasSwitchableAccount ? "/profile" : "/login", { replace: true });
    }
  }, [currentUserId, navigate]);

  const removeAccount = useCallback(
    async (accountId: string) => {
      const account = getCachedAccounts().find((item) => item.userId === accountId);
      if (!account) return;

      if (accountId === currentUserId) {
        await logout();
        removeCachedAccount(accountId);
      } else {
        if (account.refreshToken) {
          try {
            await logoutRequest(account.refreshToken);
          } catch {
            // 非当前账号的远端退出失败只影响服务端 token 回收，本地移除仍按用户显式操作执行。
          }
        }
        removeCachedAccount(accountId);
      }
      syncSnapshot();
    },
    [currentUserId, logout]
  );

  useEffect(() => {
    setAuthExpiredHandler((message) => {
      expireCurrentSession();
      syncSnapshot();
      setNotice(message);
      navigate("/profile", { replace: true });
    });
    return () => setAuthExpiredHandler(null);
  }, [navigate]);

  const value = useMemo<AuthContextValue>(
    () => ({
      currentUserId,
      accessToken,
      refreshToken,
      user,
      cachedAccounts,
      isAuthenticated: Boolean(accessToken && refreshToken),
      notice,
      setTokens,
      setUser,
      finishLogin,
      switchAccount,
      removeAccount,
      clearAuth,
      logout,
      consumeNotice: () => setNotice("")
    }),
    [
      currentUserId,
      accessToken,
      refreshToken,
      user,
      cachedAccounts,
      notice,
      setTokens,
      setUser,
      finishLogin,
      switchAccount,
      removeAccount,
      clearAuth,
      logout
    ]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return ctx;
}
