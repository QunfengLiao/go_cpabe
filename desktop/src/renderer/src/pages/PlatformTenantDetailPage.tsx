import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import { disablePlatformTenant, enablePlatformTenant, getPlatformTenant } from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { TenantSummary } from "../types";
import { formatDate, TenantStatusBadge } from "./PlatformTenantListPage";

export function PlatformTenantDetailPage() {
  const { tenantId } = useParams();
  const location = useLocation();
  const numericTenantId = Number(tenantId);
  const [tenant, setTenant] = useState<TenantSummary | null>(null);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState((location.state as { notice?: string } | null)?.notice ?? "");
  const [loading, setLoading] = useState(true);
  const [updating, setUpdating] = useState(false);

  async function loadTenant() {
    if (!numericTenantId) {
      setError("租户 ID 不合法");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      setTenant(await getPlatformTenant(numericTenantId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取租户详情失败");
    } finally {
      setLoading(false);
    }
  }

  async function setStatus(nextStatus: "enabled" | "disabled") {
    if (!tenant) return;
    setError("");
    setSuccess("");
    setUpdating(true);
    try {
      if (nextStatus === "enabled") {
        await enablePlatformTenant(tenant.tenant_id);
        setSuccess("租户已启用");
      } else {
        await disablePlatformTenant(tenant.tenant_id);
        setSuccess("租户已禁用，普通用户将无法进入该租户");
      }
      await loadTenant();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "更新租户状态失败");
    } finally {
      setUpdating(false);
    }
  }

  useEffect(() => {
    void loadTenant();
  }, [numericTenantId]);

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>租户详情</h2>
          <p>查看租户基础信息，执行启用或禁用操作</p>
        </div>
        <div className="platform-actions">
          <Link className="secondary-action" to="/platform/tenants">
            返回列表
          </Link>
          {tenant && (
            <Link className="primary-action" to={`/platform/tenants/${tenant.tenant_id}/users`}>
              管理用户
            </Link>
          )}
        </div>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      {loading ? (
        <div className="panel empty-state">正在加载租户详情...</div>
      ) : tenant ? (
        <section className="panel platform-detail">
          <div className="platform-detail-title">
            <div>
              <h3>{tenant.tenant_name}</h3>
              <p>{tenant.description || "暂无描述"}</p>
            </div>
            <TenantStatusBadge status={tenant.status} />
          </div>
          <dl className="platform-detail-grid">
            <div>
              <dt>租户 ID</dt>
              <dd>{tenant.tenant_id}</dd>
            </div>
            <div>
              <dt>租户编码</dt>
              <dd>{tenant.tenant_code}</dd>
            </div>
            <div>
              <dt>用户数</dt>
              <dd>{tenant.user_count ?? 0}</dd>
            </div>
            <div>
              <dt>Tenant Admin</dt>
              <dd>{tenant.tenant_admin_count ?? 0}</dd>
            </div>
            <div>
              <dt>创建时间</dt>
              <dd>{formatDate(tenant.created_at)}</dd>
            </div>
            <div>
              <dt>更新时间</dt>
              <dd>{formatDate(tenant.updated_at)}</dd>
            </div>
          </dl>
          <div className="platform-actions platform-actions-left">
            {tenant.status === "disabled" ? (
              <button className="primary-action" type="button" onClick={() => void setStatus("enabled")} disabled={updating}>
                {updating ? "处理中..." : "启用租户"}
              </button>
            ) : (
              <button className="secondary-action danger-action" type="button" onClick={() => void setStatus("disabled")} disabled={updating}>
                {updating ? "处理中..." : "禁用租户"}
              </button>
            )}
          </div>
        </section>
      ) : (
        <div className="panel empty-state">租户不存在。</div>
      )}
    </section>
  );
}
