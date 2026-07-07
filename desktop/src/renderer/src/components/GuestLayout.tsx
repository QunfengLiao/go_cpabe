import { Outlet, useParams } from "react-router-dom";
import { TenantBrandPanel } from "./TenantBrandPanel";
import { ThemeSwitcher } from "./ThemeSwitcher";

export function GuestLayout() {
  const { tenantCode } = useParams();

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
