import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getPlatformDashboard } from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { PlatformDashboard } from "../types";

export function PlatformDashboardPage() {
  const [summary, setSummary] = useState<PlatformDashboard | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function loadDashboard() {
    setLoading(true);
    setError("");
    try {
      setSummary(await getPlatformDashboard());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取平台统计失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadDashboard();
  }, []);

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>平台控制台</h2>
          <p>查看租户治理概览，确认平台级管理边界</p>
        </div>
        <div className="platform-actions">
          <button className="secondary-action" type="button" onClick={() => void loadDashboard()} disabled={loading}>
            刷新
          </button>
          <Link className="primary-action" to="/platform/tenants/new">
            创建租户
          </Link>
        </div>
      </div>

      <Alert type="error" message={error} />

      {loading ? (
        <div className="panel empty-state">正在加载平台统计...</div>
      ) : summary ? (
        <>
          <div className="metric-grid">
            <MetricCard label="租户总数" value={summary.tenant_count} />
            <MetricCard label="启用租户" value={summary.enabled_tenant_count} tone="success" />
            <MetricCard label="禁用租户" value={summary.disabled_tenant_count} tone="danger" />
            <MetricCard label="平台用户" value={summary.user_count} />
            <MetricCard label="租户用户关系" value={summary.tenant_user_count} />
            <MetricCard label="Tenant Admin" value={summary.tenant_admin_count} />
          </div>
          <section className="panel platform-note">
            <h3>安全边界</h3>
            <p>Platform Admin 只管理租户、成员关系和租户管理员分配，不代表可以查看主密钥、用户私钥或绕过 CP-ABE 策略解密文件。</p>
            <span className={summary.audit_enabled ? "status-badge status-enabled" : "status-badge status-disabled"}>
              {summary.audit_enabled ? "平台审计已启用" : "平台审计预留中"}
            </span>
          </section>
        </>
      ) : (
        <div className="panel empty-state">暂无平台统计。</div>
      )}
    </section>
  );
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone?: "success" | "danger" }) {
  return (
    <div className={`metric-card${tone ? ` metric-card-${tone}` : ""}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
