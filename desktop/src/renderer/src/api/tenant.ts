import type { SwitchTenantData, TenantSummary } from "../types";
import { request } from "./request";

export function listMyTenants(): Promise<{ tenants: TenantSummary[]; current_tenant_id?: number | null }> {
  return request("/me/tenants");
}

export function switchTenant(tenantId: number): Promise<SwitchTenantData> {
  return request("/me/switch-tenant", {
    method: "POST",
    body: JSON.stringify({ tenant_id: tenantId })
  });
}
