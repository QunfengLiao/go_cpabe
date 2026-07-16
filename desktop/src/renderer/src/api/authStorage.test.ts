import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  getCachedAccounts,
  getCurrentTenantId,
  getStoredTenantContext,
  migrateLegacyGlobalTenantStorage,
  saveLoginSession,
  saveRefreshedSession,
  saveStoredTenantContext,
  tenantContextFromAPI
} from "./authStorage";
import type { LoginData, RefreshData } from "../types";

const savedSessions: Array<{ accountId: string; refreshToken: string; expiresAt?: number }> = [];

class MemoryStorage implements Storage {
  private data = new Map<string, string>();

  get length(): number {
    return this.data.size;
  }

  clear(): void {
    this.data.clear();
  }

  getItem(key: string): string | null {
    return this.data.get(key) ?? null;
  }

  key(index: number): string | null {
    return Array.from(this.data.keys())[index] ?? null;
  }

  removeItem(key: string): void {
    this.data.delete(key);
  }

  setItem(key: string, value: string): void {
    this.data.set(key, value);
  }

  toJSON(): Record<string, string> {
    return Object.fromEntries(this.data.entries());
  }
}

vi.stubGlobal("localStorage", new MemoryStorage());
vi.stubGlobal("sessionStorage", new MemoryStorage());

vi.mock("./authSessionStore", () => ({
  saveAccountSession: (accountId: string, refreshToken: string, expiresAt?: number) => {
    savedSessions.push({ accountId, refreshToken, expiresAt });
    return Promise.resolve();
  }
}));

function loginData(): LoginData {
  return {
    access_token: "access-token",
    access_token_expires_in: 900,
    refresh_token: "refresh-token-secret",
    refresh_token_expires_in: 3600,
    token_type: "Bearer",
    user: {
      id: 1,
      email: "user@example.com",
      nickname: "测试用户",
      role: "data_user",
      avatar_url: "",
      bio: "",
      birthday: null,
      must_change_password: false,
      created_at: "2026-01-01T00:00:00Z"
    },
    tenants: [],
    platform_roles: []
  };
}

function refreshData(): RefreshData {
  return {
    access_token: "new-access-token",
    access_token_expires_in: 900,
    refresh_token: "new-refresh-token",
    refresh_token_expires_in: 3600,
    token_type: "Bearer"
  };
}

describe("authStorage refresh token 迁移", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    savedSessions.length = 0;
  });

  it("saveLoginSession 只把 refresh token 交给安全会话存储", async () => {
    await saveLoginSession(loginData());

    expect(localStorage.getItem("go_cpabe_access_token")).toBe("access-token");
    expect(localStorage.getItem("go_cpabe_refresh_token")).toBeNull();
    expect(localStorage.getItem("go_cpabe_account_refresh_tokens")).toBeNull();
    expect(JSON.stringify(localStorage)).not.toContain("refresh-token-secret");
    expect(savedSessions).toHaveLength(1);
    expect(savedSessions[0]).toMatchObject({ accountId: "1", refreshToken: "refresh-token-secret" });
  });

  it("旧账号中的 refreshToken 被降级为需要重新登录并清理旧 key", () => {
    localStorage.setItem("go_cpabe_refresh_token", "legacy-secret");
    localStorage.setItem("go_cpabe_account_refresh_tokens", JSON.stringify({ 1: "legacy-secret" }));
    localStorage.setItem(
      "go_cpabe_cached_accounts",
      JSON.stringify([{ userId: "1", email: "user@example.com", nickname: "旧账号", role: "data_user", refreshToken: "legacy-secret" }])
    );

    const [account] = getCachedAccounts();

    expect(account.status).toBe("login_required");
    expect(account.expired).toBe(true);
    expect(localStorage.getItem("go_cpabe_refresh_token")).toBeNull();
    expect(localStorage.getItem("go_cpabe_account_refresh_tokens")).toBeNull();
    expect(localStorage.getItem("go_cpabe_cached_accounts")).not.toContain("legacy-secret");
  });

  it("租户上下文按 userId 隔离，当前账号只读取自己的 currentTenantId", () => {
    localStorage.setItem("go_cpabe_current_user_id", "1");
    saveStoredTenantContext("1", {
      currentTenantId: "101",
      currentTenantCode: "a",
      tenants: [{ tenant_id: 101, tenant_name: "A", tenant_code: "a", roles: ["TENANT_ADMIN"] }]
    });
    saveStoredTenantContext("2", {
      currentTenantId: "202",
      currentTenantCode: "b",
      tenants: [{ tenant_id: 202, tenant_name: "B", tenant_code: "b", roles: ["DO"] }]
    });

    expect(getCurrentTenantId()).toBe("101");

    localStorage.setItem("go_cpabe_current_user_id", "2");

    expect(getCurrentTenantId()).toBe("202");
  });

  it("旧全局租户 key 会迁移到当前用户并删除旧 key", () => {
    localStorage.setItem("go_cpabe_tenants", JSON.stringify([{ tenant_id: 7, tenant_name: "旧租户", tenant_code: "legacy", roles: ["DO"] }]));
    localStorage.setItem("go_cpabe_current_tenant_id", "7");
    localStorage.setItem("go_cpabe_current_tenant_code", "legacy");

    const migrated = migrateLegacyGlobalTenantStorage("9");

    expect(migrated?.currentTenantId).toBe("7");
    expect(getStoredTenantContext("9")?.currentTenantCode).toBe("legacy");
    expect(localStorage.getItem("go_cpabe_tenants")).toBeNull();
    expect(localStorage.getItem("go_cpabe_current_tenant_id")).toBeNull();
  });

  it("saveRefreshedSession 不继承其他账号的租户上下文", () => {
    localStorage.setItem("go_cpabe_current_user_id", "1");
    saveStoredTenantContext("1", {
      currentTenantId: "101",
      currentTenantCode: "a",
      tenants: [{ tenant_id: 101, tenant_name: "A", tenant_code: "a", roles: ["TENANT_ADMIN"] }]
    });
    saveStoredTenantContext("2", {
      currentTenantId: "202",
      currentTenantCode: "b",
      tenants: [{ tenant_id: 202, tenant_name: "B", tenant_code: "b", roles: ["DO"] }]
    });

    const snapshot = saveRefreshedSession(
      { userId: "2", email: "b@example.com", nickname: "账号2", role: "data_user", lastLoginAt: Date.now(), status: "active" },
      refreshData()
    );

    expect(snapshot.currentUserId).toBe("2");
    expect(snapshot.currentTenantId).toBe("202");
    expect(snapshot.currentTenant?.tenant_code).toBe("b");
  });

  it("/me/context 解析时不使用旧 platformRoles 兜底", async () => {
    const platformLogin = loginData();
    platformLogin.user = { ...platformLogin.user, id: 8, email: "platform@example.com" };
    await saveLoginSession({ ...platformLogin, platform_roles: ["PLATFORM_ADMIN"] });
    saveStoredTenantContext("8", {
      currentTenantId: "",
      tenants: [],
      platformRoles: ["PLATFORM_ADMIN"]
    });

    const context = tenantContextFromAPI("8", { tenants: [], platform_roles: [] });

    expect(context.platformRoles).toEqual([]);
    expect(getCachedAccounts().find((account) => account.userId === "8")?.platformRoles ?? []).toEqual([]);
  });

  it("/me/context 只有一个有效租户时自动选中", () => {
    const context = tenantContextFromAPI("10", {
      tenants: [{ tenant_id: 301, tenant_name: "唯一租户", tenant_code: "only", roles: ["DO"] }],
      platform_roles: []
    });

    expect(context.currentTenantId).toBe("301");
    expect(context.currentTenantCode).toBe("only");
    expect(context.tenantRoles).toEqual(["DO"]);
  });

  it("/me/context 会把真实租户角色同步到账户切换缓存", async () => {
    const tenantAdminLogin = loginData();
    tenantAdminLogin.user = { ...tenantAdminLogin.user, id: 13, email: "tenant-admin@example.com", role: "data_user" };
    await saveLoginSession(tenantAdminLogin);

    tenantContextFromAPI("13", {
      tenants: [{ tenant_id: 601, tenant_name: "租户 A", tenant_code: "tenant-a", roles: ["TENANT_ADMIN"] }],
      platform_roles: []
    });

    const account = getCachedAccounts().find((item) => item.userId === "13");
    expect(account?.role).toBe("data_user");
    expect(account?.tenantRoles).toEqual(["TENANT_ADMIN"]);
  });

  it("旧账号缓存会从对应用户的租户上下文回填真实角色", () => {
    saveStoredTenantContext("14", {
      currentTenantId: "602",
      currentTenantCode: "tenant-b",
      tenants: [{ tenant_id: 602, tenant_name: "租户 B", tenant_code: "tenant-b", roles: ["TENANT_ADMIN"] }],
      tenantRoles: ["TENANT_ADMIN"]
    });
    localStorage.setItem(
      "go_cpabe_cached_accounts",
      JSON.stringify([{ userId: "14", email: "legacy-admin@example.com", nickname: "旧租户管理员", role: "data_user", lastLoginAt: Date.now(), status: "active" }])
    );

    const account = getCachedAccounts().find((item) => item.userId === "14");
    expect(account?.tenantRoles).toEqual(["TENANT_ADMIN"]);
  });

  it("/me/context 多个租户时优先恢复当前账号自己的有效历史选择", () => {
    saveStoredTenantContext("11", {
      currentTenantId: "402",
      currentTenantCode: "tenant-b",
      tenants: [{ tenant_id: 402, tenant_name: "租户 B", tenant_code: "tenant-b", roles: ["DU"] }]
    });

    const context = tenantContextFromAPI("11", {
      tenants: [
        { tenant_id: 401, tenant_name: "租户 A", tenant_code: "tenant-a", roles: ["DO"] },
        { tenant_id: 402, tenant_name: "租户 B", tenant_code: "tenant-b", roles: ["DU"] }
      ],
      platform_roles: []
    });

    expect(context.currentTenantId).toBe("402");
    expect(context.currentTenantCode).toBe("tenant-b");
    expect(context.tenantRoles).toEqual(["DU"]);
  });

  it("/me/context 多个租户且历史选择失效时进入租户选择状态", () => {
    saveStoredTenantContext("12", {
      currentTenantId: "999",
      currentTenantCode: "removed",
      tenants: [{ tenant_id: 999, tenant_name: "已移除租户", tenant_code: "removed", roles: ["TENANT_ADMIN"] }]
    });

    const context = tenantContextFromAPI("12", {
      tenants: [
        { tenant_id: 501, tenant_name: "租户 A", tenant_code: "tenant-a", roles: ["DO"] },
        { tenant_id: 502, tenant_name: "租户 B", tenant_code: "tenant-b", roles: ["DU"] }
      ],
      platform_roles: []
    });

    expect(context.currentTenantId).toBe("");
    expect(context.tenantRoles).toEqual([]);
  });
});
