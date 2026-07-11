import type { RuleToken } from "./tree/ruleSummary";

interface PolicyRuleSummaryProps {
  title?: string;
  tokens: RuleToken[];
  emptyText?: string;
}

export function PolicyRuleSummary({ title = "访问规则摘要", tokens, emptyText = "暂无规则" }: PolicyRuleSummaryProps) {
  return (
    <div className="policy-rule-block">
      <span>{title}</span>
      <RuleTags tokens={tokens} emptyText={emptyText} />
    </div>
  );
}

export function RuleTags({ tokens, emptyText = "暂无规则" }: { tokens: RuleToken[]; emptyText?: string }) {
  if (tokens.length === 0) {
    return (
      <div className="policy-rule-tags">
        <span className="rule-token muted">{emptyText}</span>
      </div>
    );
  }
  return (
    <div className="policy-rule-tags">
      {tokens.map((token, index) => (
        <span key={`${token.label}-${index}`} className={`rule-token rule-token-${token.type}`}>
          {token.label}
        </span>
      ))}
    </div>
  );
}
