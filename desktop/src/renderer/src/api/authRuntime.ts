import type { AuthStateSnapshot, AuthorizationStatus, TenantRole, TenantSummary, User } from "../types";

export type AuthRuntimeStatus = AuthStateSnapshot["authStatus"];

export interface AuthRuntimeSnapshot {
  currentUserId: string;
  accessToken: string;
  user: User | null;
  platformRoles: TenantRole[];
  tenants: TenantSummary[];
  currentTenantId: string;
  currentTenantCode: string;
  currentTenant: TenantSummary | null;
  tenantRoles: TenantRole[];
  permissions: string[];
  authReady: boolean;
  tenantContextReady: boolean;
  authStatus: AuthRuntimeStatus;
  authorizationStatus: AuthorizationStatus;
  authorizationUserId: string;
  authorizationTenantId: string;
  authorizationGeneration: number;
  authorizationError: string;
  generation: number;
}

const listeners = new Set<() => void>();

let runtime: AuthRuntimeSnapshot = {
  currentUserId: "",
  accessToken: "",
  user: null,
  platformRoles: [],
  tenants: [],
  currentTenantId: "",
  currentTenantCode: "",
  currentTenant: null,
  tenantRoles: [],
  permissions: [],
  authReady: true,
  tenantContextReady: true,
  authStatus: "idle",
  authorizationStatus: "idle",
  authorizationUserId: "",
  authorizationTenantId: "",
  authorizationGeneration: 0,
  authorizationError: "",
  generation: 0
};

export function getAuthRuntime(): AuthRuntimeSnapshot {
  return runtime;
}

export function setAuthRuntime(next: Partial<AuthRuntimeSnapshot>): AuthRuntimeSnapshot {
  runtime = { ...runtime, ...next };
  listeners.forEach((listener) => listener());
  return runtime;
}

export function setAuthRuntimeFromSnapshot(snapshot: AuthStateSnapshot, generation = runtime.generation): AuthRuntimeSnapshot {
  return setAuthRuntime({
    currentUserId: snapshot.currentUserId,
    accessToken: snapshot.accessToken,
    user: snapshot.user,
    platformRoles: snapshot.platformRoles,
    tenants: snapshot.tenants,
    currentTenantId: snapshot.currentTenantId,
    currentTenantCode: snapshot.currentTenantCode,
    currentTenant: snapshot.currentTenant,
    tenantRoles: snapshot.tenantRoles,
    permissions: snapshot.permissions,
    authReady: snapshot.authReady,
    tenantContextReady: snapshot.tenantContextReady,
    authStatus: snapshot.authStatus,
    authorizationStatus: "idle",
    authorizationUserId: "",
    authorizationTenantId: "",
    authorizationGeneration: generation,
    authorizationError: "",
    generation
  });
}

export async function waitForAuthReady(timeoutMs = 10000): Promise<AuthRuntimeSnapshot> {
  const current = getAuthRuntime();
  if (isSettled(current)) return current;
  return new Promise((resolve, reject) => {
    const startedAt = Date.now();
    const timer = globalThis.setInterval(() => {
      if (Date.now() - startedAt >= timeoutMs) {
        cleanup();
        reject(new Error("认证上下文初始化超时"));
      }
    }, 100);
    const listener = () => {
      const next = getAuthRuntime();
      if (!isSettled(next)) return;
      cleanup();
      resolve(next);
    };
    function cleanup() {
      globalThis.clearInterval(timer);
      listeners.delete(listener);
    }
    listeners.add(listener);
  });
}

function isSettled(snapshot: AuthRuntimeSnapshot): boolean {
  return snapshot.authReady && snapshot.tenantContextReady && !["initializing", "switching-account", "switching-tenant", "resolving-tenant"].includes(snapshot.authStatus);
}

export async function waitForAuthorizationReady(timeoutMs = 10000): Promise<AuthRuntimeSnapshot> {
  const current = getAuthRuntime();
  if (isAuthorizationSettled(current)) return current;
  return new Promise((resolve, reject) => {
    const startedAt = Date.now();
    const timer = globalThis.setInterval(() => {
      if (Date.now() - startedAt >= timeoutMs) {
        cleanup();
        reject(new Error("授权上下文初始化超时"));
      }
    }, 100);
    const listener = () => {
      const next = getAuthRuntime();
      if (!isAuthorizationSettled(next)) return;
      cleanup();
      resolve(next);
    };
    function cleanup() {
      globalThis.clearInterval(timer);
      listeners.delete(listener);
    }
    listeners.add(listener);
  });
}

export function isAuthorizationReady(snapshot: AuthRuntimeSnapshot): boolean {
  return (
    snapshot.authorizationStatus === "ready" &&
    snapshot.authorizationUserId === snapshot.currentUserId &&
    snapshot.authorizationTenantId === snapshot.currentTenantId &&
    snapshot.authorizationGeneration === snapshot.generation
  );
}

function isAuthorizationSettled(snapshot: AuthRuntimeSnapshot): boolean {
  if (!isSettled(snapshot)) return false;
  if (!snapshot.currentTenantId) return true;
  return snapshot.authorizationStatus === "ready" || snapshot.authorizationStatus === "error";
}
