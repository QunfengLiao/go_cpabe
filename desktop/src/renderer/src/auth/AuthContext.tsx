import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  clearCurrentSession,
  expireCurrentSession,
  getCachedAccounts,
  getAuthSnapshot,
  markCachedAccountExpired,
  markCachedAccountLoggedOut,
  removeCachedAccount,
  saveLoginSession,
  saveRefreshedSession,
  saveStoredTenantContext,
  saveTenantContext,
  saveTokens,
  tenantContextFromAPI,
  saveUser
} from "../api/authStorage";
import { hasAccountSession as hasSecureAccountSession, logoutAccountSession, refreshAccountSession, removeAccountSession } from "../api/authSessionStore";
import { listMyTenants, switchTenant as switchTenantRequest } from "../api/tenant";
import { getCurrentUser } from "../api/user";
import { ApiError, clearRequestCache, setAuthExpiredHandler } from "../api/request";
import { getAuthRuntime, setAuthRuntime, setAuthRuntimeFromSnapshot, type AuthRuntimeSnapshot } from "../api/authRuntime";
import { getCurrentAuthorization, rbacErrorMessage } from "../api/rbac";
import { uniquePermissions } from "./permissions";
import type { AuthorizationContextDTO, AuthorizationStatus, CachedAccount, LoginData, TenantRole, TenantSummary, User } from "../types";
import { applyTenantBranding, cacheTenantBranding, preloadTenantBranding } from "../theme/tenantBranding";

interface AuthContextValue {
  currentUserId: string;
  accessToken: string;
  user: User | null;
  tenants: TenantSummary[];
  currentTenantId: string;
  currentTenantCode: string;
  currentTenant: TenantSummary | null;
  tenantRoles: TenantRole[];
  permissions: string[];
  authorizationStatus: AuthorizationStatus;
  authorizationUserId: string;
  authorizationTenantId: string;
  authorizationGeneration: number;
  authorizationError: string;
  platformRoles: TenantRole[];
  authReady: boolean;
  tenantContextReady: boolean;
  authStatus: "idle" | "initializing" | "switching-account" | "switching-tenant" | "resolving-tenant" | "ready" | "no-tenant" | "select-tenant" | "error";
  isPlatformAdmin: boolean;
  cachedAccounts: CachedAccount[];
  isAuthenticated: boolean;
  notice: string;
  setTokens: (accessToken: string) => void;
  setUser: (user: User | null) => void;
  finishLogin: (data: LoginData) => Promise<void>;
  refreshTenantContext: () => Promise<void>;
  refreshAuthorization: () => Promise<void>;
  clearAuthorization: (status?: AuthorizationStatus, error?: string) => void;
  hasPermission: (code: string) => boolean;
  hasAnyPermission: (codes: string[]) => boolean;
  hasAllPermissions: (codes: string[]) => boolean;
  switchTenant: (tenantId: number) => Promise<void>;
  hasAccountSession: (accountId: string) => boolean;
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
  const [user, setUserState] = useState<User | null>(initial.user);
  const [tenants, setTenants] = useState<TenantSummary[]>(initial.tenants);
  const [currentTenantId, setCurrentTenantIdState] = useState(initial.currentTenantId);
  const [currentTenantCode, setCurrentTenantCodeState] = useState(initial.currentTenantCode);
  const [currentTenant, setCurrentTenantState] = useState<TenantSummary | null>(initial.currentTenant);
  const [tenantRoles, setTenantRoles] = useState<TenantRole[]>(initial.tenantRoles);
  const [permissions, setPermissions] = useState<string[]>(initial.permissions);
  const [authorizationStatus, setAuthorizationStatus] = useState<AuthorizationStatus>(initial.accessToken && initial.currentUserId && initial.currentTenantId ? "loading" : "idle");
  const [authorizationUserId, setAuthorizationUserId] = useState("");
  const [authorizationTenantId, setAuthorizationTenantId] = useState("");
  const [authorizationGeneration, setAuthorizationGeneration] = useState(0);
  const [authorizationError, setAuthorizationError] = useState("");
  const [platformRoles, setPlatformRoles] = useState<TenantRole[]>(initial.platformRoles);
  const [authReady, setAuthReady] = useState(initial.accessToken && initial.currentUserId ? false : initial.authReady);
  const [tenantContextReady, setTenantContextReady] = useState(initial.accessToken && initial.currentUserId ? false : initial.tenantContextReady);
  const [authStatus, setAuthStatus] = useState<AuthContextValue["authStatus"]>(initial.accessToken && initial.currentUserId ? "resolving-tenant" : initial.authStatus);
  const [cachedAccounts, setCachedAccounts] = useState<CachedAccount[]>(initial.cachedAccounts);
  const [notice, setNotice] = useState("");
  const authGeneration = useRef(0);
  const authorizationPromise = useRef<{ key: string; promise: Promise<void> } | null>(null);
  const startupContextResolved = useRef(false);
  const runtimeSeeded = useRef(false);
  if (!runtimeSeeded.current) {
    runtimeSeeded.current = true;
    setAuthRuntimeFromSnapshot(
      {
        ...initial,
        authReady: initial.accessToken && initial.currentUserId ? false : initial.authReady,
        tenantContextReady: initial.accessToken && initial.currentUserId ? false : initial.tenantContextReady,
        authStatus: initial.accessToken && initial.currentUserId ? "initializing" : initial.authStatus
      },
      0
    );
    setAuthRuntime({
      authorizationStatus: initial.accessToken && initial.currentUserId && initial.currentTenantId ? "loading" : "idle",
      authorizationUserId: "",
      authorizationTenantId: "",
      authorizationGeneration: 0,
      authorizationError: ""
    });
  }

  function syncSnapshot() {
    const snapshot = getAuthSnapshot();
    setCurrentUserId(snapshot.currentUserId);
    setAccessToken(snapshot.accessToken);
    setUserState(snapshot.user);
    setTenants(snapshot.tenants);
    setCurrentTenantIdState(snapshot.currentTenantId);
    setCurrentTenantCodeState(snapshot.currentTenantCode);
    setCurrentTenantState(snapshot.currentTenant);
    setTenantRoles(snapshot.tenantRoles);
    setPermissions(snapshot.permissions);
    setPlatformRoles(snapshot.platformRoles);
    setAuthReady(snapshot.authReady);
    setTenantContextReady(snapshot.tenantContextReady);
    setAuthStatus(snapshot.authStatus);
    setCachedAccounts(snapshot.cachedAccounts);
    setAuthRuntimeFromSnapshot(snapshot, authGeneration.current);
    return snapshot;
  }

  function clearTenantMemory(status: AuthContextValue["authStatus"] = "resolving-tenant", runtimeOverride: Partial<AuthRuntimeSnapshot> = {}) {
	void window.desktopEncryption?.clearContext();
    const baseRuntime = { ...getAuthRuntime(), ...runtimeOverride };
    setTenants([]);
    setCurrentTenantIdState("");
    setCurrentTenantCodeState("");
    setCurrentTenantState(null);
    setTenantRoles([]);
    setPermissions([]);
    setAuthorizationStatus("idle");
    setAuthorizationUserId("");
    setAuthorizationTenantId("");
    setAuthorizationGeneration(authGeneration.current);
    setAuthorizationError("");
    setTenantContextReady(false);
    setAuthStatus(status);
    setAuthRuntime({
      ...baseRuntime,
      tenants: [],
      currentTenantId: "",
      currentTenantCode: "",
      currentTenant: null,
      tenantRoles: [],
      permissions: [],
      authorizationStatus: "idle",
      authorizationUserId: "",
      authorizationTenantId: "",
      authorizationGeneration: authGeneration.current,
      authorizationError: "",
      tenantContextReady: false,
      authStatus: status,
      generation: authGeneration.current
    });
    applyTenantBranding(null, { clearing: true });
  }

  function writeAuthorizationState(status: AuthorizationStatus, details: Partial<Pick<AuthRuntimeSnapshot, "authorizationUserId" | "authorizationTenantId" | "authorizationGeneration" | "authorizationError" | "permissions" | "tenantRoles">> = {}) {
    const nextUserId = details.authorizationUserId ?? (status === "ready" ? getAuthRuntime().currentUserId : "");
    const nextTenantId = details.authorizationTenantId ?? (status === "ready" ? getAuthRuntime().currentTenantId : "");
    const nextGeneration = details.authorizationGeneration ?? authGeneration.current;
    const nextError = details.authorizationError ?? "";
    if (details.permissions) setPermissions(details.permissions);
    if (details.tenantRoles) setTenantRoles(details.tenantRoles);
    setAuthorizationStatus(status);
    setAuthorizationUserId(nextUserId);
    setAuthorizationTenantId(nextTenantId);
    setAuthorizationGeneration(nextGeneration);
    setAuthorizationError(nextError);
    setAuthRuntime({
      ...(details.permissions ? { permissions: details.permissions } : {}),
      ...(details.tenantRoles ? { tenantRoles: details.tenantRoles } : {}),
      authorizationStatus: status,
      authorizationUserId: nextUserId,
      authorizationTenantId: nextTenantId,
      authorizationGeneration: nextGeneration,
      authorizationError: nextError,
      generation: authGeneration.current
    });
  }

  function clearAuthorizationState(status: AuthorizationStatus = "idle", error = "") {
    setPermissions([]);
    writeAuthorizationState(status, {
      permissions: [],
      authorizationUserId: "",
      authorizationTenantId: "",
      authorizationGeneration: authGeneration.current,
      authorizationError: error
    });
  }

  async function loadAuthorization(baseSnapshot: AuthRuntimeSnapshot = getAuthRuntime()): Promise<void> {
    if (!baseSnapshot.accessToken || !baseSnapshot.currentUserId || !baseSnapshot.currentTenantId) {
      clearAuthorizationState("idle");
      return;
    }

    const generation = baseSnapshot.generation;
    const key = `${baseSnapshot.currentUserId}|${baseSnapshot.currentTenantId}|${generation}`;
    if (authorizationPromise.current?.key === key) return authorizationPromise.current.promise;

    writeAuthorizationState("loading", {
      permissions: [],
      authorizationUserId: baseSnapshot.currentUserId,
      authorizationTenantId: baseSnapshot.currentTenantId,
      authorizationGeneration: generation,
      authorizationError: ""
    });

    const promise = getCurrentAuthorization()
      .then((authorization: AuthorizationContextDTO) => {
        const latest = getAuthRuntime();
        if (
          latest.generation !== generation ||
          latest.currentUserId !== baseSnapshot.currentUserId ||
          latest.currentTenantId !== baseSnapshot.currentTenantId ||
          authGeneration.current !== generation
        ) {
          return;
        }
        const nextPermissions = uniquePermissions(authorization.permissions ?? []);
        const nextTenantRoles = (authorization.roles ?? []).map((role) => role.code).filter(Boolean) as TenantRole[];
        saveStoredTenantContext(baseSnapshot.currentUserId, {
          currentTenantId: baseSnapshot.currentTenantId,
          currentTenantCode: latest.currentTenantCode,
          tenants: latest.tenants,
          tenantRoles: nextTenantRoles,
          platformRoles: latest.platformRoles,
          permissions: nextPermissions
        });
        writeAuthorizationState("ready", {
          permissions: nextPermissions,
          tenantRoles: nextTenantRoles,
          authorizationUserId: baseSnapshot.currentUserId,
          authorizationTenantId: baseSnapshot.currentTenantId,
          authorizationGeneration: generation,
          authorizationError: ""
        });
      })
      .catch((err) => {
        const latest = getAuthRuntime();
        if (
          latest.generation !== generation ||
          latest.currentUserId !== baseSnapshot.currentUserId ||
          latest.currentTenantId !== baseSnapshot.currentTenantId ||
          authGeneration.current !== generation
        ) {
          return;
        }
        clearAuthorizationState("error", rbacErrorMessage(err, "授权上下文加载失败"));
      })
      .finally(() => {
        if (authorizationPromise.current?.key === key) {
          authorizationPromise.current = null;
        }
      });
    authorizationPromise.current = { key, promise };
    return promise;
  }

  const setTokens = useCallback((nextAccessToken: string) => {
    saveTokens(nextAccessToken);
    setAccessToken(nextAccessToken);
  }, []);

  const setUser = useCallback((nextUser: User | null) => {
    saveUser(nextUser);
    setUserState(nextUser);
    setCachedAccounts(getCachedAccounts());
  }, []);

  const clearAuth = useCallback((message?: string) => {
	void window.desktopEncryption?.clearContext();
    authGeneration.current += 1;
    clearRequestCache();
    clearCurrentSession();
    syncSnapshot();
    clearAuthorizationState("idle");
    if (message) setNotice(message);
  }, []);

  const finishLogin = useCallback(async (data: LoginData) => {
    authGeneration.current += 1;
    clearRequestCache();
    await saveLoginSession(data);
    const snapshot = syncSnapshot();
    if (snapshot.currentTenantId) {
      await loadAuthorization(getAuthRuntime());
    } else {
      clearAuthorizationState("idle");
    }
  }, []);

  const refreshTenantContext = useCallback(async () => {
    if (!accessToken || !currentUserId) return;
    setAuthStatus("resolving-tenant");
    setTenantContextReady(false);
    clearAuthorizationState("loading");
    const data = await listMyTenants();
    if (data.user) {
      saveUser(data.user);
    }
    tenantContextFromAPI(currentUserId, data);
    const snapshot = getAuthSnapshot();
    setTenants(snapshot.tenants);
    setCurrentTenantIdState(snapshot.currentTenantId);
    setCurrentTenantCodeState(snapshot.currentTenantCode);
    setCurrentTenantState(snapshot.currentTenant);
    setTenantRoles(snapshot.tenantRoles);
    setPermissions(snapshot.permissions);
    setPlatformRoles(snapshot.platformRoles);
    setAuthReady(true);
    setTenantContextReady(true);
    const nextAuthStatus = snapshot.currentTenantId ? "ready" : snapshot.tenants.length > 0 ? "select-tenant" : "no-tenant";
    setAuthStatus(nextAuthStatus);
    const runtime = setAuthRuntimeFromSnapshot({ ...snapshot, authReady: true, tenantContextReady: true, authStatus: nextAuthStatus }, authGeneration.current);
    if (snapshot.currentTenant) {
      cacheTenantBranding(snapshot.currentTenant);
      await preloadTenantBranding(snapshot.currentTenant);
    }
    applyTenantBranding(snapshot.currentTenant, { isPlatform: !snapshot.currentTenant && snapshot.platformRoles.includes("PLATFORM_ADMIN") });
    if (snapshot.currentTenantId) {
      await loadAuthorization(runtime);
    } else {
      clearAuthorizationState("idle");
    }
  }, [accessToken, currentUserId]);

  const switchTenant = useCallback(async (tenantId: number) => {
    const fallbackTenant = tenants.find((tenant) => tenant.tenant_id === tenantId) ?? null;
    setAuthStatus("switching-tenant");
    setTenantContextReady(false);
    clearAuthorizationState("loading");
    clearRequestCache();
    applyTenantBranding(null, { clearing: true });
    const data = await switchTenantRequest(tenantId);
    const nextTenant = data.currentTenant ?? data.tenant ?? fallbackTenant;
    const nextTenants = tenants.map((tenant) =>
      tenant.tenant_id === tenantId
        ? { ...tenant, ...nextTenant, roles: data.tenantRoles ?? data.roles ?? nextTenant?.roles ?? tenant.roles }
        : tenant
    );
    if (nextTenant) {
      cacheTenantBranding(nextTenant);
      await preloadTenantBranding(nextTenant);
    }
    saveTenantContext(nextTenants, data.current_tenant_id, nextTenant?.tenant_code ?? fallbackTenant?.tenant_code);
    saveStoredTenantContext(currentUserId, {
      currentTenantId: String(data.current_tenant_id),
      currentTenantCode: nextTenant?.tenant_code ?? fallbackTenant?.tenant_code ?? "",
      tenants: nextTenants,
      tenantRoles: data.tenantRoles ?? data.roles ?? nextTenant?.roles ?? [],
      platformRoles,
      permissions: data.permissions ?? []
    });
    setTenants(nextTenants);
    setCurrentTenantIdState(String(data.current_tenant_id));
    setCurrentTenantCodeState(nextTenant?.tenant_code ?? fallbackTenant?.tenant_code ?? "");
    setCurrentTenantState(nextTenant ?? null);
    const nextTenantRoles = data.tenantRoles ?? data.roles ?? nextTenant?.roles ?? [];
    setTenantRoles(nextTenantRoles);
    setPermissions([]);
    setTenantContextReady(true);
    setAuthStatus("ready");
    const runtime = setAuthRuntime({
      currentUserId,
      accessToken,
      user,
      platformRoles,
      tenants: nextTenants,
      currentTenantId: String(data.current_tenant_id),
      currentTenantCode: nextTenant?.tenant_code ?? fallbackTenant?.tenant_code ?? "",
      currentTenant: nextTenant ?? null,
      tenantRoles: nextTenantRoles,
      permissions: [],
      authReady: true,
      tenantContextReady: true,
      authStatus: "ready",
      generation: authGeneration.current
    });
    applyTenantBranding(nextTenant ?? null, { isPlatform: false });
    await loadAuthorization(runtime);
  }, [accessToken, currentUserId, platformRoles, tenants, user]);

  const hasAccountSession = useCallback((accountId: string) => {
    const account = getCachedAccounts().find((item) => item.userId === accountId);
    return Boolean(account && account.status === "active" && !account.expired && !account.loggedOut);
  }, []);

  const switchAccount = useCallback(
    async (accountId: string) => {
      const account = getCachedAccounts().find((item) => item.userId === accountId);
      const hasSession = account ? await hasSecureAccountSession(account.userId) : false;
      if (!account || !hasSession || account.expired || account.loggedOut) {
        if (account) {
          setCachedAccounts(markCachedAccountExpired(account.userId));
        }
        throw new ApiError("该账号登录已过期，请重新登录", 401);
      }

      const generation = authGeneration.current + 1;
      try {
        authGeneration.current = generation;
        setAuthReady(false);
        setAccessToken("");
        setCurrentUserId(account.userId);
        setUserState(null);
        setPlatformRoles([]);
        clearTenantMemory("switching-account", {
          currentUserId: account.userId,
          accessToken: "",
          user: null,
          platformRoles: [],
          authReady: false,
          tenantContextReady: false,
          authStatus: "switching-account",
          generation
        });
        clearRequestCache();
        setAuthRuntime({
          currentUserId: account.userId,
          accessToken: "",
          user: null,
          platformRoles: [],
          tenants: [],
          currentTenantId: "",
          currentTenantCode: "",
          currentTenant: null,
          tenantRoles: [],
          permissions: [],
          authorizationStatus: "idle",
          authorizationUserId: "",
          authorizationTenantId: "",
          authorizationGeneration: generation,
          authorizationError: "",
          authReady: false,
          tenantContextReady: false,
          authStatus: "switching-account",
          generation
        });
        const data = await refreshAccountSession(account.userId);
        if (authGeneration.current !== generation) return;
        const refreshedSnapshot = saveRefreshedSession(account, data);
        syncSnapshot();
        clearTenantMemory("resolving-tenant", {
          ...refreshedSnapshot,
          authReady: false,
          tenantContextReady: false,
          authStatus: "resolving-tenant",
          generation
        });

        try {
          const current = await getCurrentUser();
          if (authGeneration.current !== generation) return;
          saveUser(current.user);
          syncSnapshot();
          const userSnapshot = getAuthSnapshot();
          clearTenantMemory("resolving-tenant", {
            ...userSnapshot,
            authReady: false,
            tenantContextReady: false,
            authStatus: "resolving-tenant",
            generation
          });
        } catch {
          // 切换成功以 refresh 结果为准；资料刷新失败不回滚账号，资料页后续仍会重新拉取。
        }

        const context = await listMyTenants();
        if (authGeneration.current !== generation) return;
        if (context.user) {
          saveUser(context.user);
        }
        const resolvedUserId = context.user ? String(context.user.id) : account.userId;
        const storedContext = tenantContextFromAPI(resolvedUserId, context);
        const snapshot = getAuthSnapshot();
        setTenants(snapshot.tenants);
        setCurrentTenantIdState(snapshot.currentTenantId);
        setCurrentTenantCodeState(snapshot.currentTenantCode);
        setCurrentTenantState(snapshot.currentTenant);
        setTenantRoles(snapshot.tenantRoles);
        setPermissions([]);
        setPlatformRoles(snapshot.platformRoles);
        setCachedAccounts(snapshot.cachedAccounts);
        setAuthReady(true);
        setTenantContextReady(true);
        const nextAuthStatus = storedContext.currentTenantId ? "ready" : storedContext.tenants.length > 0 ? "select-tenant" : "no-tenant";
        setAuthStatus(nextAuthStatus);
        const runtime = setAuthRuntime({
          currentUserId: snapshot.currentUserId,
          accessToken: snapshot.accessToken,
          user: snapshot.user,
          platformRoles: snapshot.platformRoles,
          tenants: snapshot.tenants,
          currentTenantId: snapshot.currentTenantId,
          currentTenantCode: snapshot.currentTenantCode,
          currentTenant: snapshot.currentTenant,
          tenantRoles: snapshot.tenantRoles,
          permissions: [],
          authReady: true,
          tenantContextReady: true,
          authStatus: nextAuthStatus,
          generation
        });
        if (snapshot.currentTenant) {
          cacheTenantBranding(snapshot.currentTenant);
          await preloadTenantBranding(snapshot.currentTenant);
        }
        applyTenantBranding(snapshot.currentTenant, { isPlatform: !snapshot.currentTenant && snapshot.platformRoles.includes("PLATFORM_ADMIN") });
        if (snapshot.currentTenantId) {
          await loadAuthorization(runtime);
        } else {
          clearAuthorizationState("idle");
        }
        navigate(storedContext.currentTenantId ? "/profile" : storedContext.tenants.length > 0 ? "/select-tenant" : "/profile", { replace: true });
      } catch (err) {
        if (authGeneration.current !== generation) return;
        setAuthReady(true);
        setTenantContextReady(true);
        setAuthStatus("error");
        clearAuthorizationState("error", "账号切换失败，授权上下文已清空");
        setCachedAccounts(markCachedAccountExpired(account.userId));
        throw err instanceof ApiError ? err : new ApiError("该账号登录已过期，请重新登录", 401);
      }
    },
    [navigate]
  );

  const logout = useCallback(async () => {
    const logoutUserId = currentUserId;
    try {
      if (logoutUserId) {
        await logoutAccountSession(logoutUserId);
      }
    } finally {
      authGeneration.current += 1;
      clearRequestCache();
      if (logoutUserId) {
        markCachedAccountLoggedOut(logoutUserId);
      }
      clearCurrentSession();
      syncSnapshot();
      clearAuthorizationState("idle");
      navigate("/login", { replace: true });
    }
  }, [currentUserId, navigate]);

  const removeAccount = useCallback(
    async (accountId: string) => {
      const account = getCachedAccounts().find((item) => item.userId === accountId);
      if (!account) return;

      if (accountId === currentUserId) {
        await logout();
        await removeAccountSession(accountId);
        removeCachedAccount(accountId);
      } else {
        try {
          await logoutAccountSession(account.userId);
        } catch {
          // 非当前账号的远端退出失败只影响服务端 token 回收，本地移除仍按用户显式操作执行。
        }
        await removeAccountSession(account.userId);
        removeCachedAccount(accountId);
      }
      syncSnapshot();
    },
    [currentUserId, logout]
  );

  useEffect(() => {
    setAuthExpiredHandler((message) => {
      authGeneration.current += 1;
      clearRequestCache();
      expireCurrentSession();
      syncSnapshot();
      clearAuthorizationState("error", message);
      setNotice(message);
      navigate("/profile", { replace: true });
    });
    return () => setAuthExpiredHandler(null);
  }, [navigate]);

  useEffect(() => {
    if (!accessToken || !currentUserId || startupContextResolved.current) return;
    startupContextResolved.current = true;
    void refreshTenantContext().catch(() => {
      setAuthReady(true);
      setTenantContextReady(true);
      setAuthStatus("error");
    });
  }, [accessToken, currentUserId, refreshTenantContext]);

  useEffect(() => {
    setAuthRuntime({
      currentUserId,
      accessToken,
      user,
      platformRoles,
      tenants,
      currentTenantId,
      currentTenantCode,
      currentTenant,
      tenantRoles,
      permissions,
      authReady,
      tenantContextReady,
      authStatus,
      authorizationStatus,
      authorizationUserId,
      authorizationTenantId,
      authorizationGeneration,
      authorizationError,
      generation: authGeneration.current
    });
  }, [accessToken, authReady, authStatus, authorizationError, authorizationGeneration, authorizationStatus, authorizationTenantId, authorizationUserId, currentTenant, currentTenantCode, currentTenantId, currentUserId, permissions, platformRoles, tenantContextReady, tenantRoles, tenants, user]);

  useEffect(() => {
    applyTenantBranding(currentTenant, { isPlatform: !currentTenant && platformRoles.includes("PLATFORM_ADMIN") });
  }, [currentTenant, platformRoles]);

  const refreshAuthorization = useCallback(async () => {
    await loadAuthorization(getAuthRuntime());
  }, []);

  const clearAuthorization = useCallback((status: AuthorizationStatus = "idle", error = "") => {
    clearAuthorizationState(status, error);
  }, []);

  const hasPermission = useCallback((code: string) => {
    if (!code || authorizationStatus !== "ready") return false;
    if (authorizationUserId !== currentUserId || authorizationTenantId !== currentTenantId || authorizationGeneration !== authGeneration.current) return false;
    return permissions.includes(code);
  }, [authorizationGeneration, authorizationStatus, authorizationTenantId, authorizationUserId, currentTenantId, currentUserId, permissions]);

  const hasAnyPermission = useCallback((codes: string[]) => {
    if (!codes.length) return false;
    return codes.some((code) => hasPermission(code));
  }, [hasPermission]);

  const hasAllPermissions = useCallback((codes: string[]) => {
    if (!codes.length) return true;
    return codes.every((code) => hasPermission(code));
  }, [hasPermission]);

  const value = useMemo<AuthContextValue>(
    () => ({
      currentUserId,
      accessToken,
      user,
      tenants,
      currentTenantId,
      currentTenantCode,
      currentTenant,
      tenantRoles,
      permissions,
      authorizationStatus,
      authorizationUserId,
      authorizationTenantId,
      authorizationGeneration,
      authorizationError,
      platformRoles,
      authReady,
      tenantContextReady,
      authStatus,
      isPlatformAdmin: platformRoles.includes("PLATFORM_ADMIN"),
      cachedAccounts,
      isAuthenticated: Boolean(accessToken && currentUserId),
      notice,
      setTokens,
      setUser,
      finishLogin,
      refreshTenantContext,
      refreshAuthorization,
      clearAuthorization,
      hasPermission,
      hasAnyPermission,
      hasAllPermissions,
      switchTenant,
      hasAccountSession,
      switchAccount,
      removeAccount,
      clearAuth,
      logout,
      consumeNotice: () => setNotice("")
    }),
    [
      currentUserId,
      accessToken,
      user,
      tenants,
      currentTenantId,
      currentTenantCode,
      currentTenant,
      tenantRoles,
      permissions,
      authorizationStatus,
      authorizationUserId,
      authorizationTenantId,
      authorizationGeneration,
      authorizationError,
      platformRoles,
      authReady,
      tenantContextReady,
      authStatus,
      cachedAccounts,
      notice,
      setTokens,
      setUser,
      finishLogin,
      refreshTenantContext,
      refreshAuthorization,
      clearAuthorization,
      hasPermission,
      hasAnyPermission,
      hasAllPermissions,
      switchTenant,
      hasAccountSession,
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
