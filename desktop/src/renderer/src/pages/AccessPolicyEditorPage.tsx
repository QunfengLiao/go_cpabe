import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { listAvailableAttributes, listAvailableTemplates } from "../api/policy";
import { useAuth } from "../auth/AuthContext";
import { AccessTreeEditor } from "../components/access-policy/AccessTreeEditor";
import { mockTemplates } from "../components/access-policy/tree/mockData";
import type { PolicyAttribute, PolicyTemplate } from "../components/access-policy/tree/types";

export function AccessPolicyEditorPage() {
  const auth = useAuth();
  const params = useParams();
  const navigate = useNavigate();
  const policyId = params.policyId;
  const [attributes, setAttributes] = useState<PolicyAttribute[]>([]);
  const [templates, setTemplates] = useState<PolicyTemplate[]>(mockTemplates);
  const [attributesLoading, setAttributesLoading] = useState(false);
  const [attributesError, setAttributesError] = useState("");

  useEffect(() => {
    if (!auth.currentTenantId) return;
    let cancelled = false;
    setAttributes([]);
    setAttributesError("");
    setAttributesLoading(true);
    Promise.all([
      listAvailableAttributes(auth.currentTenantId),
      listAvailableTemplates(auth.currentTenantId)
    ]).then(([nextAttributes, nextTemplates]) => {
      if (cancelled) return;
      setAttributes(nextAttributes);
      if (nextTemplates.length > 0) setTemplates(nextTemplates);
    }).catch(() => {
      if (cancelled) return;
      setAttributes([]);
      setAttributesError("属性字典加载失败，请检查当前租户权限或稍后重试");
    }).finally(() => {
      if (!cancelled) setAttributesLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [auth.currentTenantId]);

  return (
    <div className="access-policy-editor-page">
      <AccessTreeEditor
        mode={policyId ? "edit" : "create"}
        policyId={policyId}
        tenantId={auth.currentTenantId}
        attributes={attributes}
        templates={templates}
        attributesLoading={attributesLoading}
        attributesError={attributesError}
        variant="workbench"
        onBack={() => navigate(policyId ? `/access-policies/${policyId}/edit` : "/access-policies/builder")}
      />
    </div>
  );
}
