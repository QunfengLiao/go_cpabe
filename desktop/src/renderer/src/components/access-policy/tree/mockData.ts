import type { PolicyAttribute, PolicyTemplate, PolicyTreeNode } from "./types";

export const mockAttributes: PolicyAttribute[] = [
  { id: 1, attrCode: "role", attrName: "租户角色", attrType: "enum", attrValues: ["DATA_OWNER", "TENANT_ADMIN", "DATA_VISITOR"], status: "enabled" },
  { id: 2, attrCode: "department", attrName: "部门", attrType: "enum", attrValues: ["研发部", "财务部", "安全部"], status: "enabled" },
  { id: 3, attrCode: "security_level", attrName: "安全等级", attrType: "number", status: "enabled" }
];

export const mockTree: PolicyTreeNode = {
  type: "OR",
  children: [
    {
      type: "AND",
      children: [
        { type: "LEAF", attribute: "department", operator: "=", value: "研发部" },
        { type: "LEAF", attribute: "role", operator: "=", value: "DATA_OWNER" }
      ]
    },
    { type: "LEAF", attribute: "role", operator: "=", value: "TENANT_ADMIN" }
  ]
};

export const mockTemplates: PolicyTemplate[] = [
  { id: 1, name: "数据拥有者或租户管理员可访问", description: "常用演示模板", policyTreeJson: mockTree, status: "enabled" }
];
