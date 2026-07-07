import type { PlatformDashboard, TenantAdminAssignment, TenantMember, TenantStatus, TenantSummary } from "../types";
import { request } from "./request";

export interface CreatePlatformTenantPayload {
  name: string;
  code: string;
  description?: string;
  status?: TenantStatus;
}

export function getPlatformDashboard(): Promise<PlatformDashboard> {
  return request("/platform/dashboard");
}

export async function listPlatformTenants(): Promise<TenantSummary[]> {
  const data = await request<{ tenants: TenantSummary[] }>("/platform/tenants");
  return data.tenants;
}

export async function createPlatformTenant(payload: CreatePlatformTenantPayload): Promise<TenantSummary> {
  const data = await request<{ tenant: TenantSummary }>("/platform/tenants", {
    method: "POST",
    body: JSON.stringify(payload)
  });
  return data.tenant;
}

export async function getPlatformTenant(tenantId: number): Promise<TenantSummary> {
  const data = await request<{ tenant: TenantSummary }>(`/platform/tenants/${tenantId}`);
  return data.tenant;
}

export function enablePlatformTenant(tenantId: number): Promise<{ tenant_id: number; status: TenantStatus }> {
  return request(`/platform/tenants/${tenantId}/enable`, { method: "PATCH" });
}

export function disablePlatformTenant(tenantId: number): Promise<{ tenant_id: number; status: TenantStatus }> {
  return request(`/platform/tenants/${tenantId}/disable`, { method: "PATCH" });
}

export async function listPlatformTenantUsers(tenantId: number): Promise<TenantMember[]> {
  const data = await request<{ users: TenantMember[] }>(`/platform/tenants/${tenantId}/users`);
  return data.users;
}

export function addPlatformTenantUser(tenantId: number, userId: number): Promise<TenantMember> {
  return request(`/platform/tenants/${tenantId}/users`, {
    method: "POST",
    body: JSON.stringify({ user_id: userId })
  });
}

export function removePlatformTenantUser(tenantId: number, userId: number): Promise<{ tenant_id: number; user_id: number; removed: boolean }> {
  return request(`/platform/tenants/${tenantId}/users/${userId}`, { method: "DELETE" });
}

export function assignPlatformTenantAdmin(tenantId: number, userId: number): Promise<TenantAdminAssignment> {
  return request(`/platform/tenants/${tenantId}/admins`, {
    method: "POST",
    body: JSON.stringify({ user_id: userId })
  });
}

export function removePlatformTenantAdmin(tenantId: number, userId: number): Promise<TenantAdminAssignment> {
  return request(`/platform/tenants/${tenantId}/admins/${userId}`, { method: "DELETE" });
}
