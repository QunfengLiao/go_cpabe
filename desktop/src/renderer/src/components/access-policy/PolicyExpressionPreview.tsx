import { PolicyRuleSummary } from "./PolicyRuleSummary";
import { parsePolicyExpressionToTokens, summarizePolicyTree } from "./tree/ruleSummary";
import type { PolicyAttribute, PolicyTreeNode, ValidationError } from "./tree/types";

export function PolicyExpressionPreview({
  expression,
  tree,
  attributes = [],
  errors
}: {
  expression: string;
  tree: PolicyTreeNode | null;
  attributes?: PolicyAttribute[];
  errors: ValidationError[];
}) {
  const tokens = errors.length ? [] : tree ? summarizePolicyTree(tree, attributes) : parsePolicyExpressionToTokens(expression, attributes);
  return (
    <PolicyRuleSummary
      title="访问规则摘要"
      tokens={tokens}
      emptyText={errors.length ? "请先处理校验结果中的访问树问题" : "从左侧添加节点后生成规则"}
    />
  );
}
