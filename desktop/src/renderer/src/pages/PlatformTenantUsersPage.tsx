import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  addPlatformTenantUser,
  assignPlatformTenantAdmin,
  getPlatformTenant,
  listPlatformTenantUsers,
  removePlatformTenantAdmin,
  removePlatformTenantUser
} from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { TenantMember, TenantSummary } from "../types";
import { TenantStatusBadge } from "./PlatformTenantListPage";

export function PlatformTenantUsersPage() {
  const { tenantId } = useParams();
  const numericTenantId = Number(tenantId);
  const [tenant, setTenant] = useState<TenantSummary | null>(null);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [userId, setUserId] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [loading, setLoading] = useState(true);
  const [workingKey, setWorkingKey] = useState("");

  async function loadUsers() {
    if (!numericTenantId) {
      setError("租户 ID 不合法");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [tenantData, userData] = await Promise.all([getPlatformTenant(numericTenantId), listPlatformTenantUsers(numericTenantId)]);
      setTenant(tenantData);
      setMembers(userData);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取租户用户失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadUsers();
  }, [numericTenantId]);

  const activeAdminCount = useMemo(
    () => members.filter((member) => member.member_status === "active" && member.roles.includes("TENANT_ADMIN")).length,
    [members]
  );

  async function onAddUser(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");
    const numericUserId = Number(userId);
    if (!Number.isInteger(numericUserId) || numericUserId <= 0) {
      setError("请输入有效的用户 ID");
      return;
    }
    setWorkingKey("add-user");
    try {
      await addPlatformTenantUser(numericTenantId, numericUserId);
      setUserId("");
      setSuccess("用户已加入租户");
      await loadUsers();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "加入租户失败");
    } finally {
      setWorkingKey("");
    }
  }

  async function onRemoveUser(member: TenantMember) {
    if (!window.confirm(`确认将 ${member.nickname || member.email} 移出该租户？`)) return;
    await runMemberAction(`remove-user-${member.user_id}`, "用户已移出租户", () => removePlatformTenantUser(numericTenantId, member.user_id));
  }

  async function onAssignAdmin(member: TenantMember) {
    await runMemberAction(`assign-admin-${member.user_id}`, "Tenant Admin 已分配", () => assignPlatformTenantAdmin(numericTenantId, member.user_id));
  }

  async function onRemoveAdmin(member: TenantMember) {
    if (activeAdminCount <= 1) {
      setError("不能移除最后一个有效 Tenant Admin");
      return;
    }
    await runMemberAction(`remove-admin-${member.user_id}`, "Tenant Admin 已移除", () => removePlatformTenantAdmin(numericTenantId, member.user_id));
  }

  async function runMemberAction(key: string, message: string, action: () => Promise<unknown>) {
    setError("");
    setSuccess("");
    setWorkingKey(key);
    try {
      await action();
      setSuccess(message);
      await loadUsers();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    } finally {
      setWorkingKey("");
    }
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>租户用户关系</h2>
          <p>{tenant ? `${tenant.tenant_name} 的成员和 Tenant Admin 分配` : "维护用户与租户之间的归属关系"}</p>
        </div>
        <div className="platform-actions">
          <button className="secondary-action" type="button" onClick={() => void loadUsers()} disabled={loading}>
            刷新
          </button>
          <Link className="secondary-action" to={tenant ? `/platform/tenants/${tenant.tenant_id}` : "/platform/tenants"}>
            返回详情
          </Link>
        </div>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      {tenant && (
        <section className="panel platform-tenant-strip">
          <div>
            <strong>{tenant.tenant_name}</strong>
            <span>{tenant.tenant_code}</span>
          </div>
          <TenantStatusBadge status={tenant.status} />
        </section>
      )}

      <section className="panel platform-note">
        <h3>角色分配边界</h3>
        <p>平台管理员只负责加入成员和指定 Tenant Admin，不在这里分配数据拥有者或数据访问者。</p>
      </section>

      <form className="panel compact-form" onSubmit={(event) => void onAddUser(event)}>
        <label className="field">
          <span>加入用户 ID</span>
          <input value={userId} inputMode="numeric" onChange={(event) => setUserId(event.target.value)} placeholder="输入已存在用户的 ID" />
        </label>
        <button className="primary-action" type="submit" disabled={workingKey === "add-user" || tenant?.status === "disabled"}>
          {workingKey === "add-user" ? "加入中..." : "加入租户"}
        </button>
      </form>

      <div className="panel platform-table-wrap">
        {loading ? (
          <div className="empty-state">正在加载租户用户...</div>
        ) : members.length > 0 ? (
          <table className="platform-table">
            <thead>
              <tr>
                <th>用户</th>
                <th>用户 ID</th>
                <th>成员状态</th>
                <th>租户角色</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => {
                const isAdmin = member.roles.includes("TENANT_ADMIN");
                return (
                  <tr key={member.user_id}>
                    <td>
                      <strong>{member.nickname || "未命名用户"}</strong>
                      <small>{member.email}</small>
                    </td>
                    <td>{member.user_id}</td>
                    <td><MemberStatusBadge status={member.member_status} /></td>
                    <td>{roleListLabel(member.roles)}</td>
                    <td>
                      <div className="inline-action-row">
                        {isAdmin ? (
                          <button
                            className="secondary-action"
                            type="button"
                            onClick={() => void onRemoveAdmin(member)}
                            disabled={Boolean(workingKey) || activeAdminCount <= 1}
                          >
                            {workingKey === `remove-admin-${member.user_id}` ? "移除中..." : "移除管理员"}
                          </button>
                        ) : (
                          <button
                            className="secondary-action"
                            type="button"
                            onClick={() => void onAssignAdmin(member)}
                            disabled={Boolean(workingKey) || member.member_status !== "active" || tenant?.status === "disabled"}
                          >
                            {workingKey === `assign-admin-${member.user_id}` ? "分配中..." : "设为管理员"}
                          </button>
                        )}
                        <button
                          className="secondary-action danger-action"
                          type="button"
                          onClick={() => void onRemoveUser(member)}
                          disabled={Boolean(workingKey) || (isAdmin && activeAdminCount <= 1)}
                        >
                          {workingKey === `remove-user-${member.user_id}` ? "移出中..." : "移出租户"}
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        ) : (
          <div className="empty-state">该租户暂无用户。</div>
        )}
      </div>
    </section>
  );
}

function MemberStatusBadge({ status }: { status: string }) {
  const active = status === "active";
  return <span className={`status-badge ${active ? "status-enabled" : "status-disabled"}`}>{active ? "有效" : "已停用"}</span>;
}

function roleListLabel(roles: string[]) {
  if (roles.length === 0) return "-";
  return roles.map((role) => {
    if (role === "TENANT_ADMIN") return "Tenant Admin";
    if (role === "DO") return "数据拥有者";
    if (role === "DU") return "数据访问者";
    return role;
  }).join("、");
}
