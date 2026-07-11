import { describe, expect, it } from "vitest";
import { flowToTree, treeToFlow } from "./convert";
import { mockTree } from "./mockData";
import type { PolicyTreeNode } from "./types";

describe("tree flow conversion", () => {
  it("可以从业务树转换到画布再回到业务树", () => {
    const flow = treeToFlow(mockTree);
    expect(flow.edges).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: "edge-root-root-0", source: "root", target: "root-0" }),
      expect.objectContaining({ id: "edge-root-root-1", source: "root", target: "root-1" }),
      expect.objectContaining({ id: "edge-root-0-root-0-0", source: "root-0", target: "root-0-0" })
    ]));
    const result = flowToTree(flow.nodes, flow.edges);
    expect(result.errors).toHaveLength(0);
    expect(result.tree).toEqual(mockTree);
  });

  it("拒绝多根画布结构", () => {
    const flow = treeToFlow(mockTree);
    const result = flowToTree([...flow.nodes, { id: "lonely", type: "attributeNode", position: { x: 0, y: 0 }, data: { nodeType: "LEAF", attribute: "role", operator: "=", value: "DATA_OWNER" } }], flow.edges);
    expect(result.errors[0]?.message).toContain("必须且只能有一个根节点");
  });

  it("保留树形属性稳定值字段", () => {
    const tree: PolicyTreeNode = {
      type: "LEAF",
      attribute: "department",
      operator: "belongs_to",
      value: "AI_BG",
      valueId: 101,
      valueCode: "AI_BG",
      label: "AI BG",
      path: "/AI_BG"
    };
    const flow = treeToFlow(tree);
    const result = flowToTree(flow.nodes, flow.edges);
    expect(result.tree).toEqual(tree);
  });
});
