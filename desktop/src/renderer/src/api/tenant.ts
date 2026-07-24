import type { SwitchTenantData, TenantContextData, TenantMember, TenantMemberCreateResult } from "../types";
import { request } from "./request";

export interface TenantMemberPage {
  users: TenantMember[];
  total: number;
  page: number;
  page_size: number;
}

export function listMyTenants(): Promise<TenantContextData> {
  return request("/me/context");
}

export function switchTenant(tenantId: number): Promise<SwitchTenantData> {
  return request("/me/switch-tenant", {
    method: "POST",
    skipRefresh: true,
    body: JSON.stringify({ tenant_id: tenantId })
  });
}

export function listTenantMembers(tenantId: number, page = 1, pageSize = 50): Promise<TenantMemberPage> {
  return request<TenantMemberPage>(`/tenants/${tenantId}/users?page=${page}&page_size=${pageSize}`);
}

export function createTenantMember(input: { username: string; displayName: string; email: string; phone?: string; roles: Array<"DO" | "DU"> }): Promise<TenantMemberCreateResult> {
  return request("/tenant/members", {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify({ username: input.username, display_name: input.displayName, email: input.email, phone: input.phone ?? "", roles: input.roles })
  });
}
