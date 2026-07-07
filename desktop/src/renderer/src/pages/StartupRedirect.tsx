import { Navigate } from "react-router-dom";
import { getLastTenantCode } from "../api/authStorage";
import { isKnownTenantCode } from "../config/tenantLoginConfigs";
import { loginPathForTenant, startupTenantCode } from "../tenantStartup";

export function StartupRedirect() {
  if (startupTenantCode) {
    return <Navigate to={loginPathForTenant(startupTenantCode)} replace />;
  }
  const lastTenantCode = getLastTenantCode();
  if (isKnownTenantCode(lastTenantCode)) {
    return <Navigate to={loginPathForTenant(lastTenantCode)} replace />;
  }
  return <Navigate to="/select-tenant" replace />;
}
