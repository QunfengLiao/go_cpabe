import { logicNodeIcon, operatorLabel, resolveAttributeValueLabel } from "./display";
import { attributeCode, attributeName, policyTree, type AccessPolicy, type PolicyAttribute, type PolicyOperator, type PolicyTreeNode } from "./types";

export type RuleToken = { type: "condition" | "and" | "or"; label: string };

const fallbackAttributeLabels: Record<string, string> = {
  department: "部门",
  org_role: "部门角色",
  role: "角色",
  tenant_role: "租户角色",
  security_level: "安全等级",
  data_category: "数据分类"
};

export function summarizePolicyRule(policy: AccessPolicy, attributes: PolicyAttribute[]): RuleToken[] {
  const tree = policyTree(policy);
  if (tree) return summarizePolicyTree(tree, attributes);
  return parsePolicyExpressionToTokens(policy.policyExpr ?? policy.policy_expr ?? "", attributes);
}

export function summarizePolicyTree(tree: PolicyTreeNode | null, attributes: PolicyAttribute[]): RuleToken[] {
  if (!tree) return [];
  const attrMap = attributeMap(attributes);
  return flattenTree(tree, attrMap);
}

export function parsePolicyExpressionToTokens(expression: string, attributes: PolicyAttribute[] = []): RuleToken[] {
  const normalized = expression.replace(/[()]/g, " ").replace(/\s+/g, " ").trim();
  if (!normalized) return [];
  const attrMap = attributeMap(attributes);
  return normalized.split(/\s+(AND|OR)\s+/).map((part) => part.trim()).filter(Boolean).map((part) => {
    if (part === "AND") return { type: "and", label: logicNodeIcon("AND") };
    if (part === "OR") return { type: "or", label: logicNodeIcon("OR") };
    return { type: "condition", label: formatExpressionCondition(part, attrMap) };
  });
}

function flattenTree(tree: PolicyTreeNode, attrMap: Map<string, PolicyAttribute>): RuleToken[] {
  if (tree.type === "LEAF") return [{ type: "condition", label: formatTreeCondition(tree, attrMap) }];
  return tree.children.flatMap((child, index) => {
    const tokens = flattenTree(child, attrMap);
    return index === 0 ? tokens : [{ type: tree.type === "OR" ? "or" : "and", label: logicNodeIcon(tree.type) } as RuleToken, ...tokens];
  });
}

function formatTreeCondition(node: Extract<PolicyTreeNode, { type: "LEAF" }>, attrMap: Map<string, PolicyAttribute>): string {
  const attr = attrMap.get(node.attribute);
  const attrLabel = attr ? attributeName(attr) : fallbackAttributeLabels[node.attribute] ?? node.attribute;
  const valueLabel = resolveAttributeValueLabel(attr, node.value, node.label);
  return `${attrLabel} ${operatorLabel(node.operator)} ${valueLabel}`;
}

function formatExpressionCondition(condition: string, attrMap: Map<string, PolicyAttribute>): string {
  const parsed = parseCondition(condition);
  if (!parsed) return condition;
  const attr = attrMap.get(parsed.attribute);
  const attrLabel = attr ? attributeName(attr) : fallbackAttributeLabels[parsed.attribute] ?? parsed.attribute;
  return `${attrLabel} ${operatorLabel(parsed.operator)} ${resolveAttributeValueLabel(attr, parsed.value)}`;
}

function parseCondition(condition: string): { attribute: string; operator: PolicyOperator; value: string } | null {
  const compactBelongsTo = condition.match(/^(.+?)belongs_to(.+)$/);
  if (compactBelongsTo) return { attribute: compactBelongsTo[1]!.trim(), operator: "belongs_to", value: cleanValue(compactBelongsTo[2]!) };

  const belongsTo = condition.match(/^([A-Za-z0-9_]+)\s+belongs_to\s+(.+)$/);
  if (belongsTo) return { attribute: belongsTo[1]!, operator: "belongs_to", value: cleanValue(belongsTo[2]!) };

  const comparison = condition.match(/^([A-Za-z0-9_]+)\s*(>=|<=|!=|=)\s*(.+)$/);
  if (comparison) return { attribute: comparison[1]!, operator: comparison[2] as PolicyOperator, value: cleanValue(comparison[3]!) };

  const colon = condition.match(/^([A-Za-z0-9_]+):(.+)$/);
  if (colon) return { attribute: colon[1]!, operator: "=", value: cleanValue(colon[2]!) };

  return null;
}

function cleanValue(value: string): string {
  const trimmed = value.trim();
  if ((trimmed.startsWith("\"") && trimmed.endsWith("\"")) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function attributeMap(attributes: PolicyAttribute[]): Map<string, PolicyAttribute> {
  const map = new Map<string, PolicyAttribute>();
  for (const attr of attributes) {
    const code = attributeCode(attr);
    if (code) map.set(code, attr);
  }
  return map;
}
