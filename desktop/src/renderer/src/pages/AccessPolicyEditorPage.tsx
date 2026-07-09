import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { listAvailableAttributes, listAvailableTemplates } from "../api/policy";
import { useAuth } from "../auth/AuthContext";
import { AccessTreeEditor } from "../components/access-policy/AccessTreeEditor";
import { mockAttributes, mockTemplates } from "../components/access-policy/tree/mockData";
import type { PolicyAttribute, PolicyTemplate } from "../components/access-policy/tree/types";

export function AccessPolicyEditorPage() {
  const auth = useAuth();
  const params = useParams();
  const navigate = useNavigate();
  const policyId = params.policyId;
  const [attributes, setAttributes] = useState<PolicyAttribute[]>(mockAttributes);
  const [templates, setTemplates] = useState<PolicyTemplate[]>(mockTemplates);

  useEffect(() => {
    if (!auth.currentTenantId) return;
    Promise.all([
      listAvailableAttributes(auth.currentTenantId),
      listAvailableTemplates(auth.currentTenantId)
    ]).then(([nextAttributes, nextTemplates]) => {
      if (nextAttributes.length > 0) setAttributes(nextAttributes);
      if (nextTemplates.length > 0) setTemplates(nextTemplates);
    }).catch(() => {
      setAttributes(mockAttributes);
      setTemplates(mockTemplates);
    });
  }, [auth.currentTenantId]);

  return (
    <div className="access-policy-editor-page">
      <AccessTreeEditor
        mode={policyId ? "edit" : "create"}
        policyId={policyId}
        tenantId={auth.currentTenantId}
        attributes={attributes}
        templates={templates}
        variant="workbench"
        onBack={() => navigate(policyId ? `/access-policies/${policyId}/edit` : "/access-policies/builder")}
      />
    </div>
  );
}
