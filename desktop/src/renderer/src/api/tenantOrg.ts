import { request } from "./request";
import type { PolicyAttribute } from "../components/access-policy/tree/types";

interface ListResult<T> {
  items: T[];
  total?: number;
  page?: number;
  pageSize?: number;
}

export type OrgUnitStatus = "enabled" | "disabled";
export type OrgMemberStatus = "active" | "inactive";
export type OrgPosition = "ORG_LEADER" | "DEPUTY_LEADER";

export interface OrgUnitAttributeValue {
  valueId: number;
  valueCode: string;
  valueLabel: string;
  valuePath: string;
  status: OrgUnitStatus;
}

export interface OrgUnitLeader {
  userId: number;
  username?: string;
  email: string;
  nickname: string;
}

export interface OrgUnitNode {
  id: number;
  tenantId: number;
  parentId?: number;
  code: string;
  name: string;
  path: string;
  level: number;
  sortOrder: number;
  status: OrgUnitStatus;
  attributeValue?: OrgUnitAttributeValue;
  memberCount?: number;
  leader?: OrgUnitLeader;
  deputyLeaderCount?: number;
  children: OrgUnitNode[];
}

export interface OrgMemberUnit {
  id: number;
  name: string;
  path: string;
}

export interface OrgMember {
  id: number;
  userId: number;
  username?: string;
  email: string;
  nickname: string;
  memberStatus: OrgMemberStatus;
  orgUnit: OrgMemberUnit;
  isPrimary: boolean;
  positions: OrgPosition[];
  systemRoles: string[];
}

export interface OrgMemberRelation {
  id: number;
  userId: number;
  orgUnitId: number;
  isPrimary: boolean;
  status: OrgMemberStatus;
}

export interface OrgMemberPage {
  items: OrgMember[];
  total: number;
  page: number;
  pageSize: number;
}

export interface UserAttribute {
  id: number;
  tenantId: number;
  userId: number;
  attrCode: string;
  attrName: string;
  valueId?: number;
  valueCode?: string;
  valueLabel?: string;
  valuePath?: string;
  numberValue?: number;
  sourceType: string;
  status: "active" | "inactive";
  syncedAt: string;
}

export interface CreateOrgUnitInput {
  parentId?: number;
  name: string;
  sortOrder: number;
}

export interface UpdateOrgUnitInput {
  name?: string;
  sortOrder?: number;
  status?: OrgUnitStatus;
}

export interface MoveOrgUnitInput {
  targetParentId?: number;
  sortOrder?: number;
}

export interface ListOrgMembersParams {
  keyword?: string;
  orgUnitId?: number;
  status?: "active" | "inactive" | "all";
  page?: number;
  pageSize?: number;
}

export async function listOrgTree(includeDisabled = true): Promise<OrgUnitNode[]> {
  const query = includeDisabled ? "?status=all" : "?status=enabled";
  const data = await request<ListResult<OrgUnitNode>>(`/tenant/org-units/tree${query}`);
  return data.items;
}

export async function createOrgUnit(input: CreateOrgUnitInput): Promise<OrgUnitNode> {
  const data = await request<{ orgUnit: OrgUnitNode }>("/tenant/org-units", { method: "POST", body: JSON.stringify(input) });
  return data.orgUnit;
}

export async function updateOrgUnit(id: number, input: UpdateOrgUnitInput): Promise<OrgUnitNode> {
  const data = await request<{ orgUnit: OrgUnitNode }>(`/tenant/org-units/${id}`, { method: "PUT", body: JSON.stringify(input) });
  return data.orgUnit;
}

export async function moveOrgUnit(id: number, input: MoveOrgUnitInput): Promise<void> {
  await request(`/tenant/org-units/${id}/move`, { method: "PUT", body: JSON.stringify(input) });
}

export async function deleteOrgUnit(id: number): Promise<void> {
  await request(`/tenant/org-units/${id}`, { method: "DELETE" });
}

export async function listOrgMembers(params: ListOrgMembersParams = {}): Promise<OrgMemberPage> {
  const search = new URLSearchParams();
  if (params.keyword) search.set("keyword", params.keyword);
  if (params.orgUnitId) search.set("orgUnitId", String(params.orgUnitId));
  if (params.status) search.set("status", params.status);
  if (params.page) search.set("page", String(params.page));
  if (params.pageSize) search.set("pageSize", String(params.pageSize));
  const suffix = search.toString() ? `?${search}` : "";
  const data = await request<ListResult<OrgMember>>(`/tenant/org-members${suffix}`);
  return { items: data.items, total: data.total ?? data.items.length, page: data.page ?? params.page ?? 1, pageSize: data.pageSize ?? params.pageSize ?? 20 };
}

export async function addOrgMember(userId: number, orgUnitId: number, isPrimary = false): Promise<OrgMemberRelation> {
  const data = await request<{ member: OrgMemberRelation }>("/tenant/org-members", { method: "POST", body: JSON.stringify({ userId, orgUnitId, isPrimary }) });
  return data.member;
}

export async function setOrgMemberPrimary(memberId: number): Promise<void> {
  await request(`/tenant/org-members/${memberId}/primary`, { method: "PUT", body: JSON.stringify({ primary: true }) });
}

export async function setOrgMemberPositions(memberId: number, positions: OrgPosition[]): Promise<void> {
  await request(`/tenant/org-members/${memberId}/positions`, { method: "PUT", body: JSON.stringify({ positions }) });
}

export async function removeOrgMember(memberId: number, newPrimaryMemberId?: number): Promise<void> {
  await request(`/tenant/org-members/${memberId}`, { method: "DELETE", body: JSON.stringify({ newPrimaryMemberId }) });
}

export async function listTenantPolicyAttributes(tenantId: string): Promise<PolicyAttribute[]> {
  const data = await request<ListResult<PolicyAttribute>>(`/tenants/${tenantId}/access-policy/attributes`);
  return data.items;
}

export async function syncUserAttributes(tenantId: string, userId: number): Promise<UserAttribute[]> {
  const data = await request<ListResult<UserAttribute>>(`/tenants/${tenantId}/users/${userId}/attributes/sync`, { method: "POST" });
  return data.items;
}

export async function listMyUserAttributes(tenantId: string): Promise<UserAttribute[]> {
  const data = await request<ListResult<UserAttribute>>(`/tenants/${tenantId}/users/me/attributes`);
  return data.items;
}
