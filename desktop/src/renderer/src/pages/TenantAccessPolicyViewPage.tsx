import { useEffect, useState } from "react";
import { listAccessPolicies } from "../api/policy";
import { useAuth } from "../auth/AuthContext";
import type { AccessPolicy } from "../components/access-policy/tree/types";

export function TenantAccessPolicyViewPage() {
  const auth = useAuth();
  const [items, setItems] = useState<AccessPolicy[]>([]);
  useEffect(() => {
    if (auth.currentTenantId) void listAccessPolicies(auth.currentTenantId).then(setItems);
  }, [auth.currentTenantId]);
  return (
    <div className="access-policy-page">
      <div className="page-heading"><p>访问策略</p><h2>租户策略查看</h2></div>
      <div className="policy-list">
        {items.map((policy) => <article key={policy.id} className="policy-list-item"><strong>{policy.name}</strong><code>{policy.policyExpr ?? policy.policy_expr}</code></article>)}
      </div>
    </div>
  );
}
