import { useEffect, useMemo, useState } from "react";
import { assignTenantMemberRole, listTenantMembers } from "../api/tenant";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import { TenantMemberRoleDialog } from "../components/TenantMemberRoleDialog";
import type { TenantBusinessRole, TenantMember, TenantRole } from "../types";

export function TenantMembersPage() {
  const auth = useAuth();
  const tenantId = Number(auth.currentTenantId);
  const currentTenant = useMemo(() => auth.tenants.find((tenant) => tenant.tenant_id === tenantId), [auth.tenants, tenantId]);
  const canAssignBusinessRole = Boolean(currentTenant?.roles?.includes("TENANT_ADMIN"));
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [selectedMember, setSelectedMember] = useState<TenantMember | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  async function loadMembers() {
    if (!tenantId) {
      setError("请先选择租户");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      setMembers(await listTenantMembers(tenantId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取成员列表失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadMembers();
  }, [tenantId]);

  async function onSaveRole(role: TenantBusinessRole) {
    if (!selectedMember) return;
    setSaving(true);
    setError("");
    setSuccess("");
    try {
      await assignTenantMemberRole(tenantId, selectedMember.user_id, role);
      setSelectedMember(null);
      setSuccess("成员角色已更新");
      await loadMembers();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "角色保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>租户成员</h2>
          <p>{currentTenant ? `${currentTenant.tenant_name} 的成员和普通业务角色` : "查看当前租户成员"}</p>
        </div>
        <div className="platform-actions">
          <button className="secondary-action" type="button" onClick={() => void loadMembers()} disabled={loading}>
            刷新
          </button>
        </div>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      {auth.isPlatformAdmin && (
        <section className="panel platform-note">
          <h3>平台管理员提示</h3>
          <p>平台管理员不参与租户内普通业务角色分配；需要指定租户管理员时请使用平台后台兜底能力。</p>
        </section>
      )}

      <div className="panel platform-table-wrap">
        {loading ? (
          <div className="empty-state">正在加载租户成员...</div>
        ) : members.length > 0 ? (
          <table className="platform-table">
            <thead>
              <tr>
                <th>成员</th>
                <th>用户 ID</th>
                <th>状态</th>
                <th>角色</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => (
                <tr key={member.user_id}>
                  <td>
                    <strong>{member.nickname || "未命名用户"}</strong>
                    <small>{member.email}</small>
                  </td>
                  <td>{member.user_id}</td>
                  <td><MemberStatusBadge status={member.member_status} /></td>
                  <td>{roleListLabel(member.roles)}</td>
                  <td>
                    {canAssignBusinessRole ? (
                      <button
                        className="secondary-action"
                        disabled={member.member_status !== "active" || member.roles.includes("TENANT_ADMIN")}
                        onClick={() => setSelectedMember(member)}
                        type="button"
                      >
                        {member.roles.includes("DO") || member.roles.includes("DU") ? "修改角色" : "分配角色"}
                      </button>
                    ) : (
                      <span className="muted-cell">无分配权限</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="empty-state">当前租户暂无成员。</div>
        )}
      </div>

      {selectedMember && (
        <TenantMemberRoleDialog
          member={selectedMember}
          onClose={() => setSelectedMember(null)}
          onSave={(role) => void onSaveRole(role)}
          saving={saving}
        />
      )}
    </section>
  );
}

function roleListLabel(roles: TenantRole[]): string {
  if (roles.includes("TENANT_ADMIN")) return "租户管理员";
  if (roles.includes("DO")) return "数据拥有者";
  if (roles.includes("DU")) return "数据访问者";
  return "未分配";
}

function MemberStatusBadge({ status }: { status: string }) {
  const active = status === "active";
  return <span className={`status-badge ${active ? "status-enabled" : "status-disabled"}`}>{active ? "有效" : "已停用"}</span>;
}
