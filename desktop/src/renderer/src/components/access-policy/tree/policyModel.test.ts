import { describe, expect, it } from "vitest";
import { mockAttributes, mockTree } from "./mockData";
import { addChildToPolicyTree, backendToEditableTree, createAttributePolicyNode, layoutPolicyTree, treeToFlow, validateEditablePolicyTree } from "./policyModel";

describe("policyTree 单一数据源模型", () => {
  it("从访问树生成 React Flow 节点和显式连线", () => {
    const tree = layoutPolicyTree(backendToEditableTree(mockTree));
    const flow = treeToFlow(tree);
    expect(flow.nodes).toHaveLength(5);
    expect(flow.edges).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: "edge-root-root-0", source: "root", target: "root-0" }),
      expect.objectContaining({ id: "edge-root-root-1", source: "root", target: "root-1" }),
      expect.objectContaining({ id: "edge-root-0-root-0-0", source: "root-0", target: "root-0-0" })
    ]));
  });

  it("添加子节点时只修改 policyTree.children，再由 treeToFlow 派生连线", () => {
    const tree = backendToEditableTree(mockTree);
    const child = createAttributePolicyNode("new-role", "role", "DATA_VISITOR");
    const result = addChildToPolicyTree(tree, "root", child);
    expect(result.added).toBe(true);
    expect(result.tree?.children?.some((node) => node.id === "new-role")).toBe(true);
    expect(treeToFlow(result.tree).edges).toContainEqual({ id: "edge-root-new-role", source: "root", target: "new-role" });
  });

  it("校验只返回错误，不修改原始树结构", () => {
    const tree = backendToEditableTree(mockTree);
    const before = JSON.stringify(tree);
    const errors = validateEditablePolicyTree(tree, mockAttributes);
    expect(errors).toHaveLength(0);
    expect(JSON.stringify(tree)).toBe(before);
  });
});
