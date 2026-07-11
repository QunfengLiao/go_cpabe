import { useEffect } from "react";
import { Outlet, useParams } from "react-router-dom";
import { TenantBrandPanel } from "./TenantBrandPanel";
import { ThemeSwitcher } from "./ThemeSwitcher";
import { applyTenantBranding, demoTenantBranding } from "../theme/tenantBranding";

export function GuestLayout() {
  const { tenantCode } = useParams();

  useEffect(() => {
    applyTenantBranding({
      tenant_id: 0,
      tenant_code: tenantCode ?? "common",
      tenant_name: tenantCode ?? "通用入口",
      branding: demoTenantBranding(tenantCode)
    });
  }, [tenantCode]);

  return (
    <main className="guest-shell">
      <div className="guest-appearance">
        <ThemeSwitcher />
      </div>
      {/* 游客态品牌区域只读取租户配置，不参与认证判断，避免把视觉入口和权限边界混在一起。 */}
      <TenantBrandPanel tenantCode={tenantCode} />
      <section className="guest-form-panel">
        <Outlet />
      </section>
    </main>
  );
}
