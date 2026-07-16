import { beforeEach, describe, expect, it, vi } from "vitest";
import { API_BASE_URL, clearRequestCache, request } from "./request";
import { saveStoredTenantContext } from "./authStorage";
import { setAuthRuntime } from "./authRuntime";

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
}

const fetchMock = vi.fn();

vi.stubGlobal("localStorage", new MemoryStorage());
vi.stubGlobal("sessionStorage", new MemoryStorage());
vi.stubGlobal("fetch", fetchMock);

vi.mock("./authSessionStore", () => ({
  refreshAccountSession: vi.fn()
}));

it("默认连接本地 18080 后端端口", () => {
  expect(API_BASE_URL).toBe("http://localhost:18080/api/v1");
});

function ok(body: unknown): Response {
  return {
    ok: true,
    status: 200,
    json: () => Promise.resolve({ data: body })
  } as Response;
}

function seedAccount(userId: string, tenantId: string): void {
  const tenant = { tenant_id: Number(tenantId), tenant_name: `租户${tenantId}`, tenant_code: `tenant-${tenantId}`, roles: ["TENANT_ADMIN" as const] };
  localStorage.setItem("go_cpabe_current_user_id", userId);
  localStorage.setItem("go_cpabe_access_token", `token-${userId}`);
  saveStoredTenantContext(userId, {
    currentTenantId: tenantId,
    currentTenantCode: `tenant-${tenantId}`,
    tenants: [tenant]
  });
  setAuthRuntime({
    currentUserId: userId,
    accessToken: `token-${userId}`,
    user: null,
    platformRoles: [],
    tenants: [tenant],
    currentTenantId: tenantId,
    currentTenantCode: `tenant-${tenantId}`,
    currentTenant: tenant,
    tenantRoles: ["TENANT_ADMIN"],
    permissions: [],
    authReady: true,
    tenantContextReady: true,
    authStatus: "ready",
    authorizationStatus: "ready",
    authorizationUserId: userId,
    authorizationTenantId: tenantId,
    authorizationGeneration: Number(userId),
    authorizationError: "",
    generation: Number(userId)
  });
}

describe("request 租户请求头与短缓存", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    clearRequestCache();
    fetchMock.mockReset();
    fetchMock.mockResolvedValue(ok({}));
    setAuthRuntime({
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
    });
  });

  it("/me/context 不携带 X-Tenant-Id，避免恢复流程使用旧租户", async () => {
    seedAccount("1", "101");

    await request("/me/context");

    const headers = fetchMock.mock.calls[0][1].headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer token-1");
    expect(headers.get("X-Tenant-Id")).toBeNull();
  });

  it("租户接口携带当前账号自己的 X-Tenant-Id", async () => {
    seedAccount("2", "202");

    await request("/tenant/org-units/tree");

    const headers = fetchMock.mock.calls[0][1].headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer token-2");
    expect(headers.get("X-Tenant-Id")).toBe("202");
  });

  it("旧版 /tenants/:id/users 接口也按当前账号租户上下文发送", async () => {
    seedAccount("2", "202");

    await request("/tenants/101/users");

    const headers = fetchMock.mock.calls[0][1].headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer token-2");
    expect(headers.get("X-Tenant-Id")).toBe("202");
  });

  it("GET 短缓存按 userId 和 tenantId 隔离", async () => {
    fetchMock.mockResolvedValueOnce(ok({ value: "account-1" })).mockResolvedValueOnce(ok({ value: "account-2" }));
    seedAccount("1", "101");
    const first = await request<{ value: string }>("/tenant/org-units/tree");

    seedAccount("2", "202");
    const second = await request<{ value: string }>("/tenant/org-units/tree");

    expect(first.value).toBe("account-1");
    expect(second.value).toBe("account-2");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("租户接口缺少当前租户上下文时直接阻止请求", async () => {
    setAuthRuntime({
      currentUserId: "3",
      accessToken: "token-3",
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
      authStatus: "no-tenant",
      authorizationStatus: "idle",
      authorizationUserId: "",
      authorizationTenantId: "",
      authorizationGeneration: 3,
      authorizationError: "",
      generation: 3
    });

    await expect(request("/tenant/org-units/tree")).rejects.toThrow("当前账号没有可用租户上下文");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("授权上下文失败时不继续发送租户业务请求", async () => {
    seedAccount("4", "404");
    setAuthRuntime({
      authorizationStatus: "error",
      authorizationUserId: "",
      authorizationTenantId: "",
      authorizationGeneration: 4,
      authorizationError: "授权上下文加载失败"
    });

    await expect(request("/tenant/roles")).rejects.toThrow("授权上下文加载失败");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("授权 generation 与当前账号不匹配时阻止租户业务请求", async () => {
    seedAccount("5", "505");
    setAuthRuntime({
      authorizationStatus: "ready",
      authorizationUserId: "5",
      authorizationTenantId: "505",
      authorizationGeneration: 1
    });

    await expect(request("/tenant/roles")).rejects.toThrow("授权上下文尚未就绪");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("网络或 CORS 失败时返回明确后端连接提示", async () => {
	seedAccount("6", "606");
	fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));

	await expect(request("/tenant/me/rsa-public-keys", { method: "POST", headers: { "Idempotency-Key": "test-key" }, body: "{}" }))
		.rejects.toMatchObject({ code: "NETWORK_ERROR", message: expect.stringContaining("无法连接后端服务") });
  });
});
