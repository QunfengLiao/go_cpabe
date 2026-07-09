import { describe, expect, it } from "vitest";
import { generatePolicyExpr } from "./expression";
import { validateTree } from "./validate";
import { mockAttributes, mockTree } from "./mockData";

describe("generatePolicyExpr", () => {
  it("生成带括号的访问策略表达式", () => {
    expect(generatePolicyExpr(mockTree)).toBe("(department:研发部 AND role:DATA_OWNER) OR role:TENANT_ADMIN");
  });
});

describe("validateTree", () => {
  it("接受合法访问树", () => {
    expect(validateTree(mockTree, mockAttributes)).toHaveLength(0);
  });

  it("拒绝缺少子节点的逻辑节点", () => {
    expect(validateTree({ type: "AND", children: [] }, mockAttributes)[0]?.message).toContain("至少需要两个子节点");
  });
});
