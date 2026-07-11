import type { SwitchTenantData, TenantBusinessRole, TenantContextData, TenantMember } from "../types";
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

export function assignTenantMemberRole(tenantId: number, userId: number, roleCode: TenantBusinessRole): Promise<TenantMember> {
  return request(`/tenants/${tenantId}/members/${userId}/role`, {
    method: "PUT",
    body: JSON.stringify({ roleCode })
  });
}
