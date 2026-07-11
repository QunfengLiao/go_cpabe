import { attributeCode, attributeOperators, attributeType, attributeValues, findAttributeValue, type PolicyAttribute, type PolicyTreeNode, type ValidationError } from "./types";

export function validateTree(tree: PolicyTreeNode | null, attributes: PolicyAttribute[]): ValidationError[] {
  if (!tree) return [{ path: "root", message: "访问树不能为空" }];
  const attrMap = new Map(attributes.filter((attr) => attr.status === "enabled").map((attr) => [attributeCode(attr), attr]));
  return validateNode(tree, attrMap, "root");
}

export const validatePolicyTree = validateTree;

function validateNode(node: PolicyTreeNode, attributes: Map<string, PolicyAttribute>, path: string): ValidationError[] {
  if (node.type !== "LEAF") {
    const errors: ValidationError[] = [];
    if (node.children.length < 2) {
      errors.push({ path, message: "AND/OR 逻辑节点至少需要两个子节点" });
    }
    node.children.forEach((child, index) => errors.push(...validateNode(child, attributes, `${path}.children[${index}]`)));
    return errors;
  }

  const errors: ValidationError[] = [];
  const attr = attributes.get(node.attribute);
  if (!node.attribute || !attr) {
    errors.push({ path, message: "属性未开放或不存在" });
    return errors;
  }
  if (!attributeOperators(attr).includes(node.operator)) {
    errors.push({ path, message: "操作符不适用于当前属性类型" });
  }
  if (node.value === "" || node.value === null || node.value === undefined) {
    errors.push({ path, message: "属性值不能为空" });
  }
  if (attributeType(attr) === "enum" && !attributeValues(attr).includes(String(node.value))) {
    errors.push({ path, message: "属性值不在可选值范围内" });
  }
  if (attributeType(attr) === "tree") {
    const matched = findAttributeValue(attr, String(node.value));
    if (!matched) {
      errors.push({ path, message: "部门值不在当前租户组织树中" });
    }
    if (node.valueId && matched?.valueId && node.valueId !== matched.valueId) {
      errors.push({ path, message: "部门值标识与当前租户组织树不一致" });
    }
  }
  if (attributeType(attr) === "number" && Number.isNaN(Number(node.value))) {
    errors.push({ path, message: "属性值必须是数字" });
  }
  return errors;
}
