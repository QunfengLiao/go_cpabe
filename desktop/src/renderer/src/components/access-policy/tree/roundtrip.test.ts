import { describe, expect, it } from "vitest";
import { flowToTree, treeToFlow } from "./convert";
import { generatePolicyExpr } from "./expression";
import { mockTree } from "./mockData";

describe("访问树回显", () => {
  it("保存后的 policy_tree_json 可以稳定回显为画布并生成相同表达式", () => {
    const first = treeToFlow(mockTree);
    const restored = flowToTree(first.nodes, first.edges).tree;
    expect(restored).toEqual(mockTree);
    expect(generatePolicyExpr(restored)).toBe(generatePolicyExpr(mockTree));
  });
});
