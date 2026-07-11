import { request } from "./request";
import { listTenantPolicyAttributes } from "./tenantOrg";
import type { AccessPolicy, PolicyAttribute, PolicyStatus, PolicyTemplate, PolicyTreeNode } from "../components/access-policy/tree/types";

interface ListResult<T> {
  items: T[];
  total: number;
}

interface PolicyPayload {
  name: string;
  description: string;
  policyExpr: string;
  policyTreeJson: PolicyTreeNode;
  status: PolicyStatus;
}

export async function listAvailableAttributes(tenantId: string): Promise<PolicyAttribute[]> {
  return listTenantPolicyAttributes(tenantId);
}

export async function listAvailableTemplates(tenantId: string): Promise<PolicyTemplate[]> {
  const data = await request<ListResult<PolicyTemplate>>(`/tenants/${tenantId}/access-policy/templates`);
  return data.items;
}

export async function listAccessPolicies(tenantId: string): Promise<AccessPolicy[]> {
  const data = await request<ListResult<AccessPolicy>>(`/tenants/${tenantId}/access-policies`);
  return data.items;
}

export async function createAccessPolicy(tenantId: string, payload: PolicyPayload): Promise<AccessPolicy> {
  const data = await request<{ policy: AccessPolicy }>(`/tenants/${tenantId}/access-policies`, { method: "POST", body: JSON.stringify(payload) });
  return data.policy;
}

export async function getAccessPolicy(tenantId: string, policyId: string): Promise<AccessPolicy> {
  const data = await request<{ policy: AccessPolicy }>(`/tenants/${tenantId}/access-policies/${policyId}`);
  return data.policy;
}

export async function updateAccessPolicy(tenantId: string, policyId: string, payload: PolicyPayload): Promise<AccessPolicy> {
  const data = await request<{ policy: AccessPolicy }>(`/tenants/${tenantId}/access-policies/${policyId}`, { method: "PUT", body: JSON.stringify(payload) });
  return data.policy;
}

export async function deleteAccessPolicy(tenantId: string, policyId: string): Promise<void> {
  await request(`/tenants/${tenantId}/access-policies/${policyId}`, { method: "DELETE" });
}

export async function listPlatformAttributes(): Promise<PolicyAttribute[]> {
  const data = await request<ListResult<PolicyAttribute>>("/platform/policy-attributes");
  return data.items;
}

export async function createPlatformAttribute(payload: Partial<PolicyAttribute>): Promise<PolicyAttribute> {
  const data = await request<{ attribute: PolicyAttribute }>("/platform/policy-attributes", { method: "POST", body: JSON.stringify(payload) });
  return data.attribute;
}

export async function updatePlatformAttribute(id: number, payload: Partial<PolicyAttribute>): Promise<PolicyAttribute> {
  const data = await request<{ attribute: PolicyAttribute }>(`/platform/policy-attributes/${id}`, { method: "PUT", body: JSON.stringify(payload) });
  return data.attribute;
}

export async function deletePlatformAttribute(id: number): Promise<void> {
  await request(`/platform/policy-attributes/${id}`, { method: "DELETE" });
}

export async function listPlatformTemplates(): Promise<PolicyTemplate[]> {
  const data = await request<ListResult<PolicyTemplate>>("/platform/policy-templates");
  return data.items;
}

export async function createPlatformTemplate(payload: Partial<PolicyTemplate>): Promise<PolicyTemplate> {
  const data = await request<{ template: PolicyTemplate }>("/platform/policy-templates", { method: "POST", body: JSON.stringify(payload) });
  return data.template;
}

export async function updatePlatformTemplate(id: number, payload: Partial<PolicyTemplate>): Promise<PolicyTemplate> {
  const data = await request<{ template: PolicyTemplate }>(`/platform/policy-templates/${id}`, { method: "PUT", body: JSON.stringify(payload) });
  return data.template;
}

export async function deletePlatformTemplate(id: number): Promise<void> {
  await request(`/platform/policy-templates/${id}`, { method: "DELETE" });
}
