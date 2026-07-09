import type { PolicyTreeNode } from "./types";

export function generatePolicyExpr(tree: PolicyTreeNode | null): string {
  if (!tree) return "";
  if (tree.type === "LEAF") {
    const value = String(tree.value ?? "");
    return tree.operator === "!=" ? `${tree.attribute}!=${quoteIfNeeded(value)}` : `${tree.attribute}:${quoteIfNeeded(value)}`;
  }
  return tree.children.map((child) => {
    const expr = generatePolicyExpr(child);
    return child.type === "LEAF" ? expr : `(${expr})`;
  }).join(` ${tree.type} `);
}

export const generatePolicyExpression = generatePolicyExpr;

function quoteIfNeeded(value: string): string {
  if (!value) return "\"\"";
  return /\s|[()]/.test(value) ? JSON.stringify(value) : value;
}
