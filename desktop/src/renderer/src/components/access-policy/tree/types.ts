export type PolicyStatus = "enabled" | "disabled";
export type PolicyAttributeType = "string" | "enum" | "number";
export type PolicyOperator = "=" | "!=";
export type PolicyTreeNodeType = "AND" | "OR" | "LEAF";

export interface PolicyAttribute {
  id: number;
  attr_code?: string;
  attrCode?: string;
  attr_name?: string;
  attrName?: string;
  attr_type?: PolicyAttributeType;
  attrType?: PolicyAttributeType;
  attr_values?: string[];
  attrValues?: string[];
  description?: string;
  status: PolicyStatus;
}

export interface PolicyTemplate {
  id: number;
  name: string;
  description?: string;
  policy_expr?: string;
  policyExpr?: string;
  policy_tree_json?: PolicyTreeNode;
  policyTreeJson?: PolicyTreeNode;
  status: PolicyStatus;
}

export interface AccessPolicy {
  id: number;
  tenant_id?: number;
  tenantId?: number;
  owner_id?: number;
  ownerId?: number;
  name: string;
  description?: string;
  policy_expr?: string;
  policyExpr?: string;
  policy_tree_json?: PolicyTreeNode;
  policyTreeJson?: PolicyTreeNode;
  status: PolicyStatus;
  updated_at?: string;
  updatedAt?: string;
}

export type PolicyTreeNode =
  | { type: "AND" | "OR"; children: PolicyTreeNode[] }
  | { type: "LEAF"; attribute: string; operator: PolicyOperator; value: string | number };

export interface FlowAccessNodeData extends Record<string, unknown> {
  nodeType: PolicyTreeNodeType;
  attribute?: string;
  operator?: PolicyOperator;
  value?: string | number;
  label?: string;
  error?: string;
}

export interface SimpleFlowNode {
  id: string;
  type: "andNode" | "orNode" | "attributeNode";
  position: { x: number; y: number };
  data: FlowAccessNodeData;
}

export interface SimpleFlowEdge {
  id: string;
  source: string;
  target: string;
}

export interface ValidationError {
  nodeId?: string;
  path: string;
  message: string;
}

export function attributeCode(attribute: PolicyAttribute): string {
  return attribute.attrCode ?? attribute.attr_code ?? "";
}

export function attributeName(attribute: PolicyAttribute): string {
  return attribute.attrName ?? attribute.attr_name ?? attributeCode(attribute);
}

export function attributeType(attribute: PolicyAttribute): PolicyAttributeType {
  return attribute.attrType ?? attribute.attr_type ?? "string";
}

export function attributeValues(attribute: PolicyAttribute): string[] {
  return attribute.attrValues ?? attribute.attr_values ?? [];
}

export function templateTree(template: PolicyTemplate): PolicyTreeNode | null {
  return template.policyTreeJson ?? template.policy_tree_json ?? null;
}

export function policyTree(policy: AccessPolicy): PolicyTreeNode | null {
  return policy.policyTreeJson ?? policy.policy_tree_json ?? null;
}
