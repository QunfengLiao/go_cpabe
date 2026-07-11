import { describe, expect, it } from "vitest";
import { parsePolicyExpressionToTokens, summarizePolicyTree } from "./ruleSummary";
import type { PolicyAttribute, PolicyTreeNode } from "./types";

const attributes: PolicyAttribute[] = [
  {
    id: 1,
    attrCode: "department",
    attrName: "部门",
    attrType: "tree",
    status: "enabled",
    tree: [
      { valueCode: "PROCESS_IT", label: "流程 IT 部", path: "/PROCESS_IT" },
      { valueCode: "AGENT_APP", label: "智能体应用部", path: "/AI_BG/AGENT_APP" }
    ]
  },
  {
    id: 2,
    attrCode: "org_role",
    attrName: "部门角色",
    attrType: "enum",
    status: "enabled",
    values: [{ valueCode: "ORG_MEMBER", label: "部门成员" }]
  }
];

describe("访问策略列表规则摘要", () => {
  it("从策略树生成中文摘要并保留逻辑关系", () => {
    const tree: PolicyTreeNode = {
      type: "OR",
      children: [
        {
          type: "AND",
          children: [
            { type: "LEAF", attribute: "department", operator: "=", value: "PROCESS_IT" },
            { type: "LEAF", attribute: "org_role", operator: "=", value: "ORG_MEMBER" }
          ]
        },
        { type: "LEAF", attribute: "department", operator: "belongs_to", value: "AGENT_APP" }
      ]
    };

    expect(summarizePolicyTree(tree, attributes).map((token) => token.label)).toEqual([
      "部门 等于 流程 IT 部",
      "且",
      "部门角色 等于 部门成员",
      "或",
      "部门 属于 智能体应用部"
    ]);
  });

  it("表达式兜底解析不再显示 belongs_to 和英文角色编码", () => {
    const tokens = parsePolicyExpressionToTokens("department:PROCESS_IT AND org_role:ORG_MEMBER OR department belongs_to AGENT_APP", attributes);
    expect(tokens.map((token) => token.label)).toEqual([
      "部门 等于 流程 IT 部",
      "且",
      "部门角色 等于 部门成员",
      "或",
      "部门 属于 智能体应用部"
    ]);
  });

  it("兼容旧表达式里被压紧的 belongs_to 片段", () => {
    const tokens = parsePolicyExpressionToTokens("departmentbelongs_toAGENT_APP", attributes);
    expect(tokens[0]?.label).toBe("部门 属于 智能体应用部");
  });
});
