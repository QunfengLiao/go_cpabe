import { useEffect, useState, type ReactNode } from "react";
import { Link } from "react-router-dom";
import {
  ApartmentOutlined,
  CheckCircleOutlined,
  SafetyCertificateOutlined,
  StopOutlined,
  TeamOutlined,
  UserOutlined
} from "@ant-design/icons";
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
          <Link className="primary-action" to="/platform/tenants">
            管理租户
          </Link>
        </div>
      </div>

      <Alert type="error" message={error} />

      {loading ? (
        <div className="panel empty-state">正在加载平台统计...</div>
      ) : summary ? (
        <>
          <div className="metric-grid">
            <MetricCard icon={<ApartmentOutlined />} label="租户总数" value={summary.tenant_count} />
            <MetricCard icon={<CheckCircleOutlined />} label="启用租户" value={summary.enabled_tenant_count} tone="success" />
            <MetricCard icon={<StopOutlined />} label="禁用租户" value={summary.disabled_tenant_count} tone="danger" />
            <MetricCard icon={<UserOutlined />} label="平台用户" value={summary.user_count} />
            <MetricCard icon={<TeamOutlined />} label="租户成员关系" value={summary.tenant_user_count} />
            <MetricCard icon={<SafetyCertificateOutlined />} label="Tenant Admin" value={summary.tenant_admin_count} />
          </div>
          <div className="platform-dashboard-grid">
            <section className="panel platform-quick-card">
              <div className="panel-header">
                <div>
                  <h3>快捷操作</h3>
                  <p>常用平台治理入口</p>
                </div>
              </div>
              <div className="quick-action-grid">
                <Link className="quick-action-card" to="/platform/tenants">
                  <strong>租户接入</strong>
                  <span>在租户列表中创建或查看租户</span>
                </Link>
                <Link className="quick-action-card" to="/platform/tenants">
                  <strong>查看租户</strong>
                  <span>进入租户详情和成员设置</span>
                </Link>
                <Link className="quick-action-card" to="/platform/policies">
                  <strong>策略管理</strong>
                  <span>维护平台策略模板</span>
                </Link>
              </div>
            </section>
            <section className="panel platform-note platform-note-compact">
              <div>
                <h3>安全边界</h3>
                <p>Platform Admin 只管理租户、成员关系和租户管理员分配，不代表可以查看主密钥、用户私钥或绕过 CP-ABE 策略解密文件。</p>
              </div>
              <span className={summary.audit_enabled ? "status-badge status-enabled" : "status-badge status-disabled"}>
                {summary.audit_enabled ? "平台审计已启用" : "平台审计预留中"}
              </span>
            </section>
          </div>
        </>
      ) : (
        <div className="panel empty-state">暂无平台统计。</div>
      )}
    </section>
  );
}

function MetricCard({ icon, label, value, tone }: { icon: ReactNode; label: string; value: number; tone?: "success" | "danger" }) {
  return (
    <div className={`metric-card${tone ? ` metric-card-${tone}` : ""}`}>
      <div className="metric-card-top">
        <span className="metric-icon">{icon}</span>
        <span>{label}</span>
      </div>
      <strong>{value}</strong>
    </div>
  );
}
