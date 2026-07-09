import type { EditablePolicyTreeNode } from "./policyModel";
import type { SimpleFlowEdge, SimpleFlowNode } from "./types";

export interface AccessTreeDraft {
  name: string;
  description: string;
  nodes: SimpleFlowNode[];
  edges: SimpleFlowEdge[];
  policyTree?: EditablePolicyTreeNode | null;
  selectedNodeId: string;
  remoteUpdatedAt?: string;
  savedAt: number;
}

export function draftKey(userId: string, tenantId: string, policyId = "new"): string {
  return `go_cpabe_access_policy_draft:${userId}:${tenantId}:${policyId}`;
}

export function saveDraft(key: string, draft: AccessTreeDraft): void {
  localStorage.setItem(key, JSON.stringify({ ...draft, savedAt: Date.now() }));
}

export function loadDraft(key: string): AccessTreeDraft | null {
  const raw = localStorage.getItem(key);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AccessTreeDraft;
  } catch {
    localStorage.removeItem(key);
    return null;
  }
}

export function clearDraft(key: string): void {
  localStorage.removeItem(key);
}
