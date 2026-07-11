import { describe, expect, it } from "vitest";
import { mockAttributes, mockTree } from "./mockData";
import { addChildToPolicyTree, backendToEditableTree, createAttributePolicyNode, editableToBackendTree, hydrateEditableTreeLabels, layoutPolicyTree, treeToFlow, validateEditablePolicyTree } from "./policyModel";
import type { PolicyAttribute, PolicyTreeNode } from "./types";

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

  it("用租户属性字典补全画布中文展示，同时保留稳定编码", () => {
    const attributes: PolicyAttribute[] = [{
      id: 20,
      attrCode: "department",
      attrName: "部门",
      attrType: "tree",
      tree: [{ valueCode: "AI_PLATFORM", valueId: 2001, label: "AI 平台部", path: "/AI_BG/AI_PLATFORM" }],
      status: "enabled"
    }];
    const sourceTree: PolicyTreeNode = { type: "LEAF", attribute: "department", operator: "belongs_to", value: "AI_PLATFORM" };
    const editable = hydrateEditableTreeLabels(backendToEditableTree(sourceTree), attributes);
    const flow = treeToFlow(editable, new Map(), new Map(attributes.map((attr) => [attr.attrCode ?? "", attr])));

    expect(flow.nodes[0]?.data.displayValue).toBe("AI 平台部");
    expect(editableToBackendTree(editable)).toEqual({
      type: "LEAF",
      attribute: "department",
      operator: "belongs_to",
      value: "AI_PLATFORM",
      valueId: 2001,
      valueCode: "AI_PLATFORM",
      label: "AI 平台部",
      path: "/AI_BG/AI_PLATFORM"
    });
  });
});
