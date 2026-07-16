import type { SwitchTenantData, TenantContextData, TenantMember, TenantMemberCreateResult } from "../types";
import { request } from "./request";

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

export async function listTenantMembers(tenantId: number): Promise<TenantMember[]> {
  const data = await request<{ users: TenantMember[] }>(`/tenants/${tenantId}/users`);
  return data.users;
}

export function createTenantMember(input: { username: string; displayName: string; email: string; phone?: string; roles: Array<"DO" | "DU"> }): Promise<TenantMemberCreateResult> {
  return request("/tenant/members", {
    method: "POST",
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify({ username: input.username, display_name: input.displayName, email: input.email, phone: input.phone ?? "", roles: input.roles })
  });
}
