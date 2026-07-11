import { describe, expect, it } from "vitest";
import { generatePolicyExpr } from "./expression";
import { validateTree } from "./validate";
import { mockAttributes, mockTree } from "./mockData";
import type { PolicyAttribute, PolicyTreeNode } from "./types";

describe("generatePolicyExpr", () => {
  it("生成带括号的访问策略表达式", () => {
    expect(generatePolicyExpr(mockTree)).toBe("(department:研发部 AND role:DATA_OWNER) OR role:TENANT_ADMIN");
  });

  it("生成树形归属和数字比较表达式", () => {
    const tree: PolicyTreeNode = {
      type: "AND",
      children: [
        { type: "LEAF", attribute: "department", operator: "belongs_to", value: "AI_BG", valueId: 101, valueCode: "AI_BG", label: "AI BG", path: "/AI_BG" },
        { type: "LEAF", attribute: "security_level", operator: ">=", value: 3 }
      ]
    };
    expect(generatePolicyExpr(tree)).toBe("department belongs_to AI_BG AND security_level >= 3");
  });
});

describe("validateTree", () => {
  it("接受合法访问树", () => {
    expect(validateTree(mockTree, mockAttributes)).toHaveLength(0);
  });

  it("拒绝缺少子节点的逻辑节点", () => {
    expect(validateTree({ type: "AND", children: [] }, mockAttributes)[0]?.message).toContain("至少需要两个子节点");
  });

  it("校验租户树形属性和数字比较", () => {
    const attrs: PolicyAttribute[] = [
      { id: 1, attrCode: "department", attrName: "部门", attrType: "tree", status: "enabled", operators: ["belongs_to", "="], tree: [{ valueId: 101, valueCode: "AI_BG", label: "AI BG", path: "/AI_BG" }] },
      { id: 2, attrCode: "security_level", attrName: "安全等级", attrType: "number", status: "enabled", operators: [">=", "<=", "="] }
    ];
    expect(validateTree({ type: "LEAF", attribute: "department", operator: "belongs_to", value: "AI_BG", valueId: 101 }, attrs)).toHaveLength(0);
    expect(validateTree({ type: "LEAF", attribute: "security_level", operator: ">=", value: 3 }, attrs)).toHaveLength(0);
    expect(validateTree({ type: "LEAF", attribute: "department", operator: "belongs_to", value: "CS_SCHOOL" }, attrs)[0]?.message).toContain("部门值");
  });
});
