import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { listPlatformTenants } from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { TenantSummary } from "../types";

export function PlatformTenantListPage() {
  const [tenants, setTenants] = useState<TenantSummary[]>([]);
  const [keyword, setKeyword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function loadTenants() {
    setLoading(true);
    setError("");
    try {
      setTenants(await listPlatformTenants());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取租户列表失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadTenants();
  }, []);

  const filteredTenants = useMemo(() => {
    const value = keyword.trim().toLowerCase();
    if (!value) return tenants;
    return tenants.filter((tenant) => {
      return [tenant.tenant_name, tenant.tenant_code, tenant.description].some((item) => item?.toLowerCase().includes(value));
    });
  }, [keyword, tenants]);

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>租户列表</h2>
          <p>管理平台下的组织边界、启用状态和租户管理员数量</p>
        </div>
        <div className="platform-actions">
          <button className="secondary-action" type="button" onClick={() => void loadTenants()} disabled={loading}>
            刷新
          </button>
          <Link className="primary-action" to="/platform/tenants/new">
            创建租户
          </Link>
        </div>
      </div>

      <Alert type="error" message={error} />

      <div className="panel platform-filter">
        <label className="field">
          <span>搜索租户</span>
          <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="按名称、编码或描述搜索" />
        </label>
      </div>

      <div className="panel platform-table-wrap">
        {loading ? (
          <div className="empty-state">正在加载租户列表...</div>
        ) : filteredTenants.length > 0 ? (
          <table className="platform-table">
            <thead>
              <tr>
                <th>租户名称</th>
                <th>编码</th>
                <th>状态</th>
                <th>用户数</th>
                <th>Tenant Admin</th>
                <th>创建时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredTenants.map((tenant) => (
                <tr key={tenant.tenant_id}>
                  <td>
                    <strong>{tenant.tenant_name}</strong>
                    <small>{tenant.description || "暂无描述"}</small>
                  </td>
                  <td>{tenant.tenant_code}</td>
                  <td><TenantStatusBadge status={tenant.status} /></td>
                  <td>{tenant.user_count ?? 0}</td>
                  <td>{tenant.tenant_admin_count ?? 0}</td>
                  <td>{formatDate(tenant.created_at)}</td>
                  <td>
                    <div className="inline-action-row">
                      <Link className="secondary-action" to={`/platform/tenants/${tenant.tenant_id}`}>
                        详情
                      </Link>
                      <Link className="secondary-action" to={`/platform/tenants/${tenant.tenant_id}/users`}>
                        用户
                      </Link>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="empty-state">没有匹配的租户。</div>
        )}
      </div>
    </section>
  );
}

export function TenantStatusBadge({ status }: { status?: string }) {
  const enabled = status === "enabled";
  return <span className={`status-badge ${enabled ? "status-enabled" : "status-disabled"}`}>{enabled ? "启用" : "禁用"}</span>;
}

export function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}
