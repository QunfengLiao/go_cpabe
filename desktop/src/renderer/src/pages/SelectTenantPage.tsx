import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getStoredTenants } from "../api/authStorage";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import { ThemeSwitcher } from "../components/ThemeSwitcher";
import { tenantLoginConfigs } from "../config/tenantLoginConfigs";
import type { TenantSummary } from "../types";

export function SelectTenantPage() {
  const auth = useAuth();
  const { isAuthenticated, refreshTenantContext } = auth;
  const navigate = useNavigate();
  const [tenants, setTenants] = useState<TenantSummary[]>(() => getStoredTenants());
  const [loading, setLoading] = useState(false);
  const [switchingId, setSwitchingId] = useState<number | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isAuthenticated) return;
    let cancelled = false;
    setLoading(true);
    refreshTenantContext()
      .then(() => {
        if (!cancelled) setTenants(getStoredTenants());
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "查询租户列表失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isAuthenticated, refreshTenantContext]);

  const tenantCards = useMemo(() => Object.values(tenantLoginConfigs), []);

  async function onSwitchTenant(tenant: TenantSummary) {
    setError("");
    setSwitchingId(tenant.tenant_id);
    try {
      await auth.switchTenant(tenant.tenant_id);
      navigate("/profile", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "切换租户失败");
    } finally {
      setSwitchingId(null);
    }
  }

  return (
    <main className="tenant-select-page">
      <div className="guest-appearance">
        <ThemeSwitcher />
      </div>
      <section className="tenant-select-header">
        <span className="auth-kicker">Tenant Workspace</span>
        <h1>{auth.isAuthenticated ? "选择当前租户" : "选择租户登录入口"}</h1>
        <p>{auth.isAuthenticated ? "进入租户后，后端会以该租户上下文校验后续业务数据边界。" : "不同租户使用同一套登录组件，由租户配置驱动品牌、配色和入口文案。"}</p>
      </section>
      <Alert type="error" message={error} />
      {auth.isAuthenticated ? (
        <section className="tenant-card-grid" aria-label="我的租户">
          {loading && <div className="empty-state">正在加载租户列表...</div>}
          {!loading && tenants.length === 0 && <div className="empty-state">当前账号没有可进入的租户，请联系管理员加入租户。</div>}
          {tenants.map((tenant) => (
            <TenantCard
              key={tenant.tenant_id}
              description={`角色：${tenant.roles?.join("、") || "暂无角色"}`}
              disabled={tenant.status === "disabled" || switchingId === tenant.tenant_id}
              name={tenant.tenant_name}
              code={tenant.tenant_code}
              status={tenant.status === "disabled" ? "已禁用" : switchingId === tenant.tenant_id ? "切换中..." : "进入"}
              onClick={() => void onSwitchTenant(tenant)}
            />
          ))}
        </section>
      ) : (
        <section className="tenant-card-grid" aria-label="租户登录入口">
          {tenantCards.map((tenant) => (
            <TenantCard
              key={tenant.code}
              description={tenant.subtitle}
              name={tenant.name}
              code={tenant.code}
              status="登录"
              to={`/login/${tenant.code}`}
            />
          ))}
          <TenantCard description="不指定租户，登录后由用户所属租户决定下一步。" name="通用登录入口" code="common" status="登录" to="/login" />
        </section>
      )}
    </main>
  );
}

function TenantCard({
  code,
  description,
  disabled,
  name,
  onClick,
  status,
  to
}: {
  code: string;
  description: string;
  disabled?: boolean;
  name: string;
  onClick?: () => void;
  status: string;
  to?: string;
}) {
  const content = (
    <>
      <span className="tenant-card-code">{code}</span>
      <strong>{name}</strong>
      <small>{description}</small>
      <em>{status}</em>
    </>
  );

  if (to) {
    return (
      <Link className="tenant-card" to={to}>
        {content}
      </Link>
    );
  }

  return (
    <button className="tenant-card" disabled={disabled} onClick={onClick} type="button">
      {content}
    </button>
  );
}
