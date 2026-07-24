import { type FormEvent, useEffect, useMemo, useState } from "react";
import { Pagination } from "antd";
import { createTenantMember, listTenantMembers } from "../api/tenant";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import { TenantMemberRoleDialog } from "../components/TenantMemberRoleDialog";
import { TenantImportDrawer } from "../components/tenant-import/TenantImportDrawer";
import type { MemberRoleDTO, TenantMember, TenantRole } from "../types";

export function memberCreationSuccessMessage(result: { created_user: boolean; temporary_password?: string }): string {
  return result.created_user ? `成员已创建，初始密码：${result.temporary_password ?? "lqf999.."}；首次登录后会提示尽快修改密码。` : "已有账号已加入当前租户，原密码和资料未被修改。";
}

export function TenantMembersPage() {
  const auth = useAuth();
  const tenantId = Number(auth.currentTenantId);
  const currentTenant = useMemo(() => auth.tenants.find((tenant) => tenant.tenant_id === tenantId), [auth.tenants, tenantId]);
  const canAssignBusinessRole = auth.hasPermission("tenant.member.manage");
  const canImport = auth.hasPermission("tenant.import.manage");
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [memberPage, setMemberPage] = useState({ page: 1, pageSize: 50, total: 0 });
  const [selectedMember, setSelectedMember] = useState<TenantMember | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState({ username: "", displayName: "", email: "", phone: "", roles: ["DU"] as Array<"DO" | "DU"> });

  async function loadMembers(page = 1, pageSize = memberPage.pageSize) {
    if (!tenantId) {
      setError("请先选择租户");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const result = await listTenantMembers(tenantId, page, pageSize);
      setMembers(result.users);
      setMemberPage({ page: result.page, pageSize: result.page_size, total: result.total });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取成员列表失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadMembers(1);
  }, [tenantId]);

  async function onRolesSaved(result: MemberRoleDTO) {
    if (!selectedMember) return;
    setError("");
    setSuccess("");
    try {
      if (String(selectedMember.user_id) === auth.currentUserId) {
        await auth.refreshAuthorization();
      }
      setSelectedMember(null);
      setSuccess("成员角色已更新");
      setMembers((current) => current.map((member) => member.user_id === result.userId ? { ...member, roles: result.roles.map((role) => role.code) } : member));
      await loadMembers(memberPage.page);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "角色刷新失败");
    }
  }

  async function onCreateMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(""); setSuccess("");
    if (createForm.roles.length === 0) { setError("请至少选择一个初始能力角色"); return; }
    setCreating(true);
    try {
      const result = await createTenantMember(createForm);
      setCreateOpen(false);
      setCreateForm({ username: "", displayName: "", email: "", phone: "", roles: ["DU"] });
      setSuccess(memberCreationSuccessMessage(result));
      await loadMembers();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "新增成员失败");
    } finally {
      setCreating(false);
    }
  }

  function toggleCreateRole(role: "DO" | "DU") {
    setCreateForm((form) => ({ ...form, roles: form.roles.includes(role) ? form.roles.filter((item) => item !== role) : [...form.roles, role] }));
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>租户成员</h2>
          <p>{currentTenant ? `${currentTenant.tenant_name} 的成员、治理角色、能力角色和自定义角色` : "查看当前租户成员"}</p>
        </div>
        <div className="platform-actions">
          {canAssignBusinessRole && <button className="primary-action" type="button" onClick={() => setCreateOpen(true)}>新增成员</button>}
          {canImport && <button className="secondary-action" type="button" onClick={() => setImportOpen(true)}>批量导入</button>}
          <button className="secondary-action" type="button" onClick={() => void loadMembers(memberPage.page)} disabled={loading}>
            刷新
          </button>
        </div>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      {auth.isPlatformAdmin && (
        <section className="panel platform-note">
          <h3>平台管理员提示</h3>
          <p>平台管理员不参与租户内成员角色分配；租户内操作权限由角色绑定的 permission 决定。</p>
        </section>
      )}

      <div className="panel platform-table-wrap">
        {loading ? (
          <div className="empty-state">正在加载租户成员...</div>
        ) : members.length > 0 ? (
          <>
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
                        disabled={member.member_status !== "active"}
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
            <Pagination
              current={memberPage.page}
              pageSize={memberPage.pageSize}
              total={memberPage.total}
              showSizeChanger={false}
              showTotal={(total) => `共 ${total} 名成员`}
              onChange={(page) => void loadMembers(page)}
            />
          </>
        ) : (
          <div className="empty-state">当前租户暂无成员。</div>
        )}
      </div>

      {selectedMember && (
        <TenantMemberRoleDialog
          member={selectedMember}
          onClose={() => setSelectedMember(null)}
          onSaved={(result) => void onRolesSaved(result)}
        />
      )}

      {createOpen && (
        <div className="platform-modal-backdrop" role="presentation" onClick={() => !creating && setCreateOpen(false)}>
          <section className="platform-modal" role="dialog" aria-modal="true" aria-labelledby="create-tenant-member-title" onClick={(event) => event.stopPropagation()}>
            <div className="modal-title-row">
              <div><span>成员管理</span><strong id="create-tenant-member-title">新增租户成员</strong></div>
              <button className="secondary-action" type="button" disabled={creating} onClick={() => setCreateOpen(false)}>关闭</button>
            </div>
            <form className="platform-modal-form" onSubmit={(event) => void onCreateMember(event)}>
              <label className="field"><span>用户名</span><input required minLength={3} maxLength={64} value={createForm.username} onChange={(event) => setCreateForm((form) => ({ ...form, username: event.target.value }))} placeholder="例如 du.zhangsan" autoFocus /></label>
              <label className="field"><span>姓名</span><input required maxLength={20} value={createForm.displayName} onChange={(event) => setCreateForm((form) => ({ ...form, displayName: event.target.value }))} placeholder="成员姓名" /></label>
              <label className="field"><span>邮箱</span><input required type="email" value={createForm.email} onChange={(event) => setCreateForm((form) => ({ ...form, email: event.target.value }))} placeholder="用于登录；已有邮箱会复用账号" /></label>
              <label className="field"><span>手机号（可选）</span><input maxLength={32} value={createForm.phone} onChange={(event) => setCreateForm((form) => ({ ...form, phone: event.target.value }))} /></label>
              <fieldset className="drawer-form-section">
                <legend>初始能力角色</legend>
                <label className="role-dialog-checkbox"><input type="checkbox" checked={createForm.roles.includes("DO")} onChange={() => toggleCreateRole("DO")} /><span><strong>数据拥有者 DO</strong><small>负责数据加密、策略绑定和文件发布等能力</small></span></label>
                <label className="role-dialog-checkbox"><input type="checkbox" checked={createForm.roles.includes("DU")} onChange={() => toggleCreateRole("DU")} /><span><strong>数据使用者 DU</strong><small>负责访问和解密符合权限的文件，可与 DO 同时承担</small></span></label>
              </fieldset>
              <p className="drawer-subtitle">新账号初始密码统一为 <strong>lqf999..</strong>，首次登录会提示尽快修改；已有账号不会重置密码。</p>
              <div className="modal-action-row"><button className="secondary-action" type="button" disabled={creating} onClick={() => setCreateOpen(false)}>取消</button><button className="primary-action" type="submit" disabled={creating}>{creating ? "创建中..." : "创建成员"}</button></div>
            </form>
          </section>
        </div>
      )}
      <TenantImportDrawer open={importOpen} type="users" onClose={() => setImportOpen(false)} onCompleted={() => loadMembers(memberPage.page)} />
    </section>
  );
}

function roleListLabel(roles: TenantRole[]): string {
  if (!roles.length) return "未分配";
  return roles.map(roleDisplayName).join(" / ");
}

function roleDisplayName(role: TenantRole): string {
  if (role === "TENANT_ADMIN") return "租户管理员";
  if (role === "DO") return "数据拥有者 DO";
  if (role === "DU") return "数据使用者 DU";
  return role;
}

function MemberStatusBadge({ status }: { status: string }) {
  const active = status === "active";
  return <span className={`status-badge ${active ? "status-enabled" : "status-disabled"}`}>{active ? "有效" : "已停用"}</span>;
}
