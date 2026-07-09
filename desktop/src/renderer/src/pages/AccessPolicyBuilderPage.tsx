import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { getAccessPolicy, listAccessPolicies, listAvailableAttributes } from "../api/policy";
import { useAuth } from "../auth/AuthContext";
import { AccessTreePreviewCanvas } from "../components/access-policy/AccessTreePreviewCanvas";
import { PolicyPreviewTabs } from "../components/access-policy/PolicyPreviewTabs";
import { flowToTree, treeToFlow } from "../components/access-policy/tree/convert";
import { generatePolicyExpr } from "../components/access-policy/tree/expression";
import { applyAutoLayout } from "../components/access-policy/tree/layout";
import { mockAttributes, mockTree } from "../components/access-policy/tree/mockData";
import { validateTree } from "../components/access-policy/tree/validate";
import { policyTree, type AccessPolicy, type PolicyAttribute, type PolicyTreeNode, type SimpleFlowEdge, type SimpleFlowNode } from "../components/access-policy/tree/types";

export function AccessPolicyBuilderPage() {
  const auth = useAuth();
  const params = useParams();
  const navigate = useNavigate();
  const policyId = params.policyId;
  const initial = useMemo(() => treeToFlow(mockTree), []);
  const [policies, setPolicies] = useState<AccessPolicy[]>([]);
  const [selectedPolicyId, setSelectedPolicyId] = useState(policyId ?? "");
  const [name, setName] = useState("研发部数据拥有者访问策略");
  const [description, setDescription] = useState("研发部数据拥有者或租户管理员可访问");
  const [updatedAt, setUpdatedAt] = useState("");
  const [nodes, setNodes] = useState<SimpleFlowNode[]>(() => applyAutoLayout(initial.nodes, initial.edges));
  const [edges, setEdges] = useState<SimpleFlowEdge[]>(() => initial.edges);
  const [attributes, setAttributes] = useState<PolicyAttribute[]>(mockAttributes);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (!auth.currentTenantId) return;
    Promise.all([
      listAvailableAttributes(auth.currentTenantId),
      listAccessPolicies(auth.currentTenantId)
    ]).then(([nextAttributes, nextPolicies]) => {
      if (nextAttributes.length > 0) setAttributes(nextAttributes);
      setPolicies(nextPolicies);
      const nextSelectedId = policyId ?? String(nextPolicies[0]?.id ?? "");
      setSelectedPolicyId(nextSelectedId);
      const selectedPolicy = nextPolicies.find((policy) => String(policy.id) === nextSelectedId);
      if (selectedPolicy) applyPolicyToPreview(selectedPolicy);
    }).catch(() => setAttributes(mockAttributes));
  }, [auth.currentTenantId]);

  useEffect(() => {
    if (!policyId) return;
    setSelectedPolicyId(policyId);
  }, [policyId]);

  useEffect(() => {
    if (!selectedPolicyId || !auth.currentTenantId) return;
    const loadedPolicy = policies.find((policy) => String(policy.id) === selectedPolicyId);
    if (loadedPolicy) {
      applyPolicyToPreview(loadedPolicy);
      return;
    }
    getAccessPolicy(auth.currentTenantId, selectedPolicyId).then(applyPolicyToPreview).catch(() => setMessage("策略详情加载失败，已展示示例访问树"));
  }, [auth.currentTenantId, policies, selectedPolicyId]);

  const conversion = useMemo(() => flowToTree(nodes, edges), [nodes, edges]);
  const expression = useMemo(() => generatePolicyExpr(conversion.tree), [conversion.tree]);
  const validationErrors = useMemo(() => [...conversion.errors, ...validateTree(conversion.tree, attributes)], [attributes, conversion.errors, conversion.tree]);
  const summary = useMemo(() => summarizeTree(conversion.tree, validationErrors.length, updatedAt), [conversion.tree, updatedAt, validationErrors.length]);
  const editorPath = selectedPolicyId ? `/access-policies/${selectedPolicyId}/edit/tree` : "/access-policies/builder/editor";

  function applyPolicyToPreview(policy: AccessPolicy) {
    setMessage("");
    setName(policy.name);
    setDescription(policy.description ?? "");
    setUpdatedAt(policy.updatedAt ?? policy.updated_at ?? "");
    const tree = policyTree(policy);
    if (tree) {
      const flow = treeToFlow(tree);
      setNodes(applyAutoLayout(flow.nodes, flow.edges));
      setEdges(flow.edges);
    } else {
      setNodes([]);
      setEdges([]);
    }
  }

  function onSelectPolicy(nextPolicyId: string) {
    setSelectedPolicyId(nextPolicyId);
    if (nextPolicyId) navigate(`/access-policies/${nextPolicyId}/edit`, { replace: true });
  }

  function openEditor() {
    navigate(editorPath);
  }

  return (
    <div className="access-policy-page access-policy-summary-page">
      <header className="policy-summary-header">
        <div>
          <span>访问策略详情</span>
          <h2>{name}</h2>
          <p>{description || "暂无策略描述"}</p>
        </div>
        <div className="policy-summary-actions">
          <label className="policy-selector">
            <span>选择访问策略</span>
            <select value={selectedPolicyId} onChange={(event) => onSelectPolicy(event.target.value)}>
              <option value="">请选择访问策略</option>
              {policies.map((policy) => (
                <option key={policy.id} value={String(policy.id)}>{policy.name}</option>
              ))}
            </select>
          </label>
          <button className="policy-editor-entry" type="button" onClick={openEditor}>进入可视化编辑器</button>
        </div>
      </header>

      {message && <div className="access-tree-message">{message}</div>}

      <section className="policy-summary-grid">
        <article className="policy-summary-card policy-basic-card">
          <div className="summary-card-title">
            <span>基础信息</span>
            <strong>策略配置</strong>
          </div>
          <label>
            策略名称
            <input value={name} readOnly />
          </label>
          <label>
            策略描述
            <textarea value={description} readOnly rows={4} />
          </label>

          <div className="policy-metric-grid">
            {summary.map((item) => (
              <div className="policy-metric-item" key={item.label}>
                <span>{item.label}</span>
                <strong>{item.value}</strong>
              </div>
            ))}
          </div>
        </article>

        <article className="policy-summary-card policy-preview-card">
          <div className="summary-card-title">
            <span>访问树预览</span>
            <strong>最终访问结构</strong>
          </div>
          <AccessTreePreviewCanvas nodes={nodes} edges={edges} errors={validationErrors} attributes={attributes} onOpenEditor={openEditor} />
        </article>
      </section>

      <PolicyPreviewTabs expression={expression} tree={conversion.tree} errors={validationErrors} />
    </div>
  );
}

function summarizeTree(tree: PolicyTreeNode | null, errorCount: number, updatedAt: string) {
  const stats = countTree(tree);
  return [
    { label: "策略类型", value: "CP-ABE 访问树" },
    { label: "根节点类型", value: tree?.type ?? "未配置" },
    { label: "逻辑节点数量", value: String(stats.logicCount) },
    { label: "属性条件数量", value: String(stats.attributeCount) },
    { label: "校验状态", value: errorCount > 0 ? `${errorCount} 个问题` : "校验通过" },
    { label: "最近保存时间", value: updatedAt ? new Date(updatedAt).toLocaleString() : "本地草稿" }
  ];
}

function countTree(tree: PolicyTreeNode | null): { logicCount: number; attributeCount: number } {
  if (!tree) return { logicCount: 0, attributeCount: 0 };
  if (tree.type === "LEAF") return { logicCount: 0, attributeCount: 1 };
  return tree.children.reduce((stats, child) => {
    const childStats = countTree(child);
    return {
      logicCount: stats.logicCount + childStats.logicCount,
      attributeCount: stats.attributeCount + childStats.attributeCount
    };
  }, { logicCount: 1, attributeCount: 0 });
}
