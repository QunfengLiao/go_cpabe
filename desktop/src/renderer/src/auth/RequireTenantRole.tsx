import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "./AuthContext";
import type { TenantRole } from "../types";

export function RequireTenantRole({ roles }: { roles: TenantRole[] }) {
  const auth = useAuth();
  const currentTenant = auth.tenants.find((tenant) => String(tenant.tenant_id ?? tenant.tenantId) === auth.currentTenantId);
  const allowed = currentTenant?.roles?.some((role) => roles.includes(role));
  if (!allowed) return <Navigate to="/profile" replace />;
  return <Outlet />;
}
