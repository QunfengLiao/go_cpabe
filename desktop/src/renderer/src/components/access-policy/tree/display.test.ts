import { describe, expect, it } from "vitest";
import { mockAttributes } from "./mockData";
import { operatorLabel, resolveAttributeValueLabel } from "./display";

describe("访问树中文展示文案", () => {
  it("优先使用属性字典中的中文值", () => {
    const orgRole = {
      id: 10,
      attrCode: "org_role",
      attrName: "部门角色",
      attrType: "enum" as const,
      values: [{ valueCode: "ORG_MANAGER", label: "部门主管" }],
      status: "enabled" as const
    };

    expect(resolveAttributeValueLabel(orgRole, "ORG_MANAGER")).toBe("部门主管");
  });

  it("字典尚未加载时用内置中文兜底展示常见角色", () => {
    expect(resolveAttributeValueLabel(undefined, "TENANT_ADMIN")).toBe("租户管理员");
    expect(resolveAttributeValueLabel(undefined, "DATA_OWNER")).toBe("数据拥有者");
  });

  it("操作符展示为中文", () => {
    expect(operatorLabel("belongs_to")).toBe("属于");
    expect(operatorLabel(">=")).toBe("大于等于");
  });

  it("不改变未知值，避免误改稳定 code", () => {
    expect(resolveAttributeValueLabel(mockAttributes[0], "CUSTOM_CODE")).toBe("CUSTOM_CODE");
  });
});
