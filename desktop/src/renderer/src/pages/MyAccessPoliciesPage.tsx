import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { deleteAccessPolicy, listAccessPolicies, listAvailableAttributes } from "../api/policy";
import { useAuth } from "../auth/AuthContext";
import { PolicyRuleSummary } from "../components/access-policy/PolicyRuleSummary";
import { summarizePolicyRule, parsePolicyExpressionToTokens } from "../components/access-policy/tree/ruleSummary";
import type { AccessPolicy, PolicyAttribute } from "../components/access-policy/tree/types";

type FilterValue = "all" | "enabled" | "draft";
type DisplayPolicyStatus = "enabled" | "draft";

export function MyAccessPoliciesPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [items, setItems] = useState<AccessPolicy[]>([]);
  const [message, setMessage] = useState("");
  const [keyword, setKeyword] = useState("");
  const [filter, setFilter] = useState<FilterValue>("all");
  const [filterOpen, setFilterOpen] = useState(false);
  const [previewPolicy, setPreviewPolicy] = useState<AccessPolicy | null>(null);
  const [attributes, setAttributes] = useState<PolicyAttribute[]>([]);
  const canWritePolicy = auth.hasPermission("policy.write");

  async function load() {
    if (!auth.currentTenantId) return;
    const policies = await listAccessPolicies(auth.currentTenantId);
    setItems(policies);
    try {
      setAttributes(await listAvailableAttributes(auth.currentTenantId));
    } catch {
      setAttributes([]);
    }
  }

  useEffect(() => { void load(); }, [auth.currentTenantId]);

  const filteredItems = useMemo(() => filterPolicies(items, keyword, filter), [filter, items, keyword]);
  const enabledCount = items.filter((policy) => policyStatus(policy) === "enabled").length;
  const latestUpdated = latestUpdatedLabel(items);

  async function onDelete(policy: AccessPolicy) {
    if (!auth.currentTenantId) return;
    if (!window.confirm(`确认删除访问策略「${policy.name}」吗？删除后不可恢复。`)) return;
    try {
      await deleteAccessPolicy(auth.currentTenantId, String(policy.id));
      setMessage("策略已删除");
      await load();
    } catch {
      setMessage("删除失败，请稍后重试");
    }
  }

  return (
    <div className="access-policy-page my-policy-page">
      <header className="my-policy-header">
        <div className="my-policy-title">
          <span>访问策略</span>
          <h2>我的访问策略</h2>
          <p>管理当前账号创建的访问控制策略，可用于加密文件访问授权</p>
        </div>
        <div className="my-policy-actions">
          {canWritePolicy && <button className="primary-policy-action" type="button" onClick={() => navigate("/access-policies/builder")}>新建策略</button>}
          <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="搜索策略名称或描述" />
          <div className="policy-filter-menu">
            <button className="secondary-policy-action" type="button" onClick={() => setFilterOpen((open) => !open)}>筛选：{filterLabel(filter)}</button>
            {filterOpen && (
              <div className="policy-filter-popover">
                {(["all", "enabled", "draft"] as FilterValue[]).map((value) => (
                  <button key={value} type="button" className={filter === value ? "active" : ""} onClick={() => { setFilter(value); setFilterOpen(false); }}>
                    {filterLabel(value)}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </header>

      {message && <div className="access-tree-message">{message}</div>}

      <section className="policy-overview-grid">
        <OverviewCard title="策略总数" value={String(items.length)} desc="当前账号创建的访问策略" />
        <OverviewCard title="已启用" value={String(enabledCount)} desc="可用于文件加密授权" />
        <OverviewCard title="最近更新" value={latestUpdated} desc="最近一次策略变更时间" />
      </section>

      {filteredItems.length === 0 ? (
        <PolicyEmptyState canCreate={canWritePolicy} hasKeyword={keyword.trim() !== "" || filter !== "all"} onCreate={() => navigate("/access-policies/builder")} />
      ) : (
        <section className="policy-card-grid">
          {filteredItems.map((policy) => (
            <article key={policy.id} className="policy-management-card">
              <div className="policy-card-top">
                <div>
                  <div className="policy-name-row">
                    <h3>{policy.name}</h3>
                    <span className={`policy-status-tag policy-status-${policyStatus(policy)}`}>{statusLabel(policyStatus(policy))}</span>
                  </div>
                  <p>{policy.description || "暂无策略描述"}</p>
                </div>
                <div className="policy-card-actions">
                  {canWritePolicy && <Link to={`/access-policies/${policy.id}/edit`}>编辑</Link>}
                  <button type="button" onClick={() => setPreviewPolicy(policy)}>预览</button>
                  {canWritePolicy && <button className="danger" type="button" onClick={() => void onDelete(policy)}>删除</button>}
                </div>
              </div>

              <PolicyRuleSummary tokens={summarizePolicyRule(policy, attributes)} />

              <div className="policy-card-meta">
                <span>更新时间：{formatDate(policyUpdatedAt(policy))}</span>
                <span>节点数量：{estimateNodeCount(policyExpression(policy))}</span>
                <span>创建人：当前账号</span>
                <span>所属租户：当前租户</span>
              </div>
            </article>
          ))}
        </section>
      )}

      {previewPolicy && <PolicyPreviewModal policy={previewPolicy} attributes={attributes} onClose={() => setPreviewPolicy(null)} />}
    </div>
  );
}

function OverviewCard({ title, value, desc }: { title: string; value: string; desc: string }) {
  return (
    <article className="policy-overview-card">
      <span>{title}</span>
      <strong>{value}</strong>
      <p>{desc}</p>
    </article>
  );
}

function PolicyEmptyState({ canCreate, hasKeyword, onCreate }: { canCreate: boolean; hasKeyword: boolean; onCreate: () => void }) {
  return (
    <section className="policy-empty-state">
      <div className="policy-empty-icon">∅</div>
      <h3>{hasKeyword ? "没有匹配的访问策略" : "暂无访问策略"}</h3>
      <p>{hasKeyword ? "请调整搜索关键词或筛选条件后再试" : "创建访问策略后，可用于控制加密文件的访问权限"}</p>
      {!hasKeyword && canCreate && <button className="primary-policy-action" type="button" onClick={onCreate}>新建访问策略</button>}
    </section>
  );
}

function PolicyPreviewModal({ policy, attributes, onClose }: { policy: AccessPolicy; attributes: PolicyAttribute[]; onClose: () => void }) {
  const expression = policyExpression(policy);
  return (
    <div className="policy-preview-modal-backdrop" role="presentation" onClick={onClose}>
      <section className="policy-preview-modal" role="dialog" aria-modal="true" aria-label="访问策略预览" onClick={(event) => event.stopPropagation()}>
        <div className="modal-title-row">
          <div>
            <span>策略预览</span>
            <strong>{policy.name}</strong>
          </div>
          <button type="button" onClick={onClose}>关闭</button>
        </div>
        <p>{policy.description || "暂无策略描述"}</p>
        <PolicyRuleSummary title="规则摘要" tokens={summarizePolicyRule(policy, attributes)} />
        <div className="policy-expression-full">{summarizePolicyRule(policy, attributes).map((token) => token.label).join(" ") || expression || "暂无表达式"}</div>
      </section>
    </div>
  );
}

function filterPolicies(items: AccessPolicy[], keyword: string, filter: FilterValue): AccessPolicy[] {
  const query = keyword.trim().toLowerCase();
  return items.filter((policy) => {
    if (filter === "enabled" && policyStatus(policy) !== "enabled") return false;
    if (filter === "draft" && policyStatus(policy) === "enabled") return false;
    if (!query) return true;
    return `${policy.name} ${policy.description ?? ""}`.toLowerCase().includes(query);
  });
}

function policyExpression(policy: AccessPolicy): string {
  return policy.policyExpr ?? policy.policy_expr ?? "";
}

function policyStatus(policy: AccessPolicy): DisplayPolicyStatus {
  return policy.status === "enabled" ? "enabled" : "draft";
}

function policyUpdatedAt(policy: AccessPolicy): string {
  return policy.updatedAt ?? policy.updated_at ?? "";
}

function statusLabel(status: DisplayPolicyStatus): string {
  if (status === "enabled") return "已启用";
  return "草稿";
}

function filterLabel(filter: FilterValue): string {
  if (filter === "enabled") return "已启用";
  if (filter === "draft") return "草稿";
  return "全部";
}

function formatDate(value: string): string {
  if (!value) return "暂无";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function latestUpdatedLabel(items: AccessPolicy[]): string {
  const latest = items.map(policyUpdatedAt).filter(Boolean).sort((a, b) => new Date(b).getTime() - new Date(a).getTime())[0];
  if (!latest) return "暂无";
  const date = new Date(latest);
  if (Number.isNaN(date.getTime())) return "暂无";
  const today = new Date();
  return date.toDateString() === today.toDateString() ? "今天" : date.toLocaleDateString();
}

function estimateNodeCount(expression: string): string {
  const count = parsePolicyExpressionToTokens(expression).filter((token) => token.type === "condition").length;
  return count > 0 ? String(count) : "未统计";
}
