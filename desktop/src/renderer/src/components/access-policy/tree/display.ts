import { attributeTree, attributeType, findAttributeValue, structuredAttributeValues, type FlowAccessNodeData, type PolicyAttribute, type PolicyAttributeType, type PolicyAttributeValue, type PolicyOperator } from "./types";

const fallbackValueLabels: Record<string, string> = {
  ORG_MANAGER: "部门主管",
  ORG_MEMBER: "部门成员",
  DATA_OWNER: "数据拥有者",
  DATA_VISITOR: "数据访问者",
  TENANT_ADMIN: "租户管理员",
  DO: "数据拥有者",
  DU: "数据访问者",
  AI_BG: "AI BG",
  AI_PLATFORM: "AI 平台部",
  AGENT_APP: "智能体应用部",
  PROCESS_IT: "流程 IT 部"
};

const operatorLabels: Record<PolicyOperator, string> = {
  "=": "等于",
  "!=": "不等于",
  ">=": "大于等于",
  "<=": "小于等于",
  belongs_to: "属于"
};

const attributeTypeLabels: Record<PolicyAttributeType | "attr", string> = {
  enum: "枚举",
  number: "数值",
  string: "文本",
  tree: "树形",
  attr: "属性"
};

export function logicNodeTitle(type: "AND" | "OR" | string | undefined): string {
  return type === "OR" ? "任一满足" : "全部满足";
}

export function logicNodeIcon(type: "AND" | "OR" | string | undefined): string {
  return type === "OR" ? "或" : "且";
}

export function logicNodeDescription(type: "AND" | "OR" | string | undefined): string {
  return type === "OR" ? "满足任一子条件" : "满足全部子条件";
}

export function operatorLabel(operator: PolicyOperator | undefined): string {
  if (!operator) return "等于";
  return operatorLabels[operator] ?? operator;
}

export function attributeTypeLabel(type: unknown): string {
  const key = typeof type === "string" ? type : "attr";
  return attributeTypeLabels[key as PolicyAttributeType | "attr"] ?? "属性";
}

export function resolveAttributeValueLabel(attribute: PolicyAttribute | undefined, value: unknown, explicitLabel?: string): string {
  if (explicitLabel) return explicitLabel;
  if (value === "" || value === undefined || value === null) return "未填写";
  const rawValue = String(value);
  const matched = attribute ? findAttributeValue(attribute, rawValue) : undefined;
  if (matched?.label) return matched.label;
  return fallbackValueLabels[rawValue] ?? rawValue;
}

export function withDisplayData(node: FlowAccessNodeData, attribute: PolicyAttribute | undefined): FlowAccessNodeData {
  if (node.nodeType !== "LEAF") return node;
  const valueLabel = resolveAttributeValueLabel(attribute, node.value, typeof node.label === "string" ? node.label : undefined);
  return {
    ...node,
    attributeType: attribute ? attributeType(attribute) : node.attributeType,
    valueLabel,
    displayValue: valueLabel,
    operatorLabel: operatorLabel(node.operator)
  };
}

export function firstAttributeValue(attribute: PolicyAttribute): { valueCode: string; valueId?: number; label?: string; path?: string } | undefined {
  const values = attributeType(attribute) === "tree" ? attributeTree(attribute) : structuredAttributeValues(attribute);
  const firstStructured = firstValue(values);
  if (firstStructured) return firstStructured;
  const rawValues = attribute.attrValues ?? attribute.attr_values ?? [];
  const rawValue = rawValues[0];
  return rawValue ? { valueCode: rawValue, label: fallbackValueLabels[rawValue] ?? rawValue } : undefined;
}

function firstValue(values: PolicyAttributeValue[]): PolicyAttributeValue | undefined {
  for (const value of values) {
    if (value.valueCode) return value;
    const child = firstValue(value.children ?? []);
    if (child) return child;
  }
  return undefined;
}
