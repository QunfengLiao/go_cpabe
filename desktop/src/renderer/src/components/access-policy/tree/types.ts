export type PolicyStatus = "enabled" | "disabled";
export type PolicyAttributeType = "string" | "enum" | "number" | "tree";
export type PolicyOperator = "=" | "!=" | ">=" | "<=" | "belongs_to";
export type PolicyTreeNodeType = "AND" | "OR" | "LEAF";

export interface PolicyAttributeValue {
  id?: number;
  valueId?: number;
  valueCode: string;
  label: string;
  path?: string;
  children?: PolicyAttributeValue[];
}

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
  valueSource?: string;
  operators?: PolicyOperator[];
  values?: PolicyAttributeValue[];
  tree?: PolicyAttributeValue[];
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
  | {
    type: "LEAF";
    attribute: string;
    operator: PolicyOperator;
    value: string | number;
    valueId?: number;
    valueCode?: string;
    label?: string;
    path?: string;
  };

export interface FlowAccessNodeData extends Record<string, unknown> {
  nodeType: PolicyTreeNodeType;
  attribute?: string;
  operator?: PolicyOperator;
  value?: string | number;
  valueId?: number;
  valueCode?: string;
  path?: string;
  label?: string;
  displayValue?: string;
  valueLabel?: string;
  operatorLabel?: string;
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
  const structured = attribute.values ?? attribute.tree ?? [];
  if (structured.length > 0) return flattenAttributeValues(structured).map((value) => value.valueCode);
  return attribute.attrValues ?? attribute.attr_values ?? [];
}

export function structuredAttributeValues(attribute: PolicyAttribute): PolicyAttributeValue[] {
  return attribute.values ?? [];
}

export function attributeTree(attribute: PolicyAttribute): PolicyAttributeValue[] {
  return attribute.tree ?? [];
}

export function attributeOperators(attribute: PolicyAttribute): PolicyOperator[] {
  if (attribute.operators && attribute.operators.length > 0) return attribute.operators;
  const type = attributeType(attribute);
  if (type === "number") return [">=", "<=", "="];
  if (type === "tree") return ["belongs_to", "="];
  return ["=", "!="];
}

export function findAttributeValue(attribute: PolicyAttribute, code: string): PolicyAttributeValue | undefined {
  const values = attributeType(attribute) === "tree" ? attributeTree(attribute) : structuredAttributeValues(attribute);
  return flattenAttributeValues(values).find((value) => value.valueCode === code);
}

export function templateTree(template: PolicyTemplate): PolicyTreeNode | null {
  return template.policyTreeJson ?? template.policy_tree_json ?? null;
}

export function policyTree(policy: AccessPolicy): PolicyTreeNode | null {
  return policy.policyTreeJson ?? policy.policy_tree_json ?? null;
}

function flattenAttributeValues(values: PolicyAttributeValue[]): PolicyAttributeValue[] {
  return values.flatMap((value) => [value, ...flattenAttributeValues(value.children ?? [])]);
}
