import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "./AuthContext";
import { ForbiddenPage } from "../pages/ForbiddenPage";

interface RequirePermissionProps {
  permission?: string;
  anyPermissions?: string[];
  allPermissions?: string[];
}

export function RequirePermission({ permission, anyPermissions, allPermissions }: RequirePermissionProps) {
  const auth = useAuth();
  const location = useLocation();

  if (!auth.isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  }
  if (!auth.currentTenantId) {
    return <Navigate to="/select-tenant" replace />;
  }
  if (auth.authorizationStatus === "loading" || auth.authStatus === "resolving-tenant" || auth.authStatus === "switching-tenant" || auth.authStatus === "switching-account") {
    return <div className="route-loading">正在恢复授权上下文...</div>;
  }
  if (auth.authorizationStatus === "error") {
    return <ForbiddenPage mode="error" message={auth.authorizationError || "授权上下文加载失败"} />;
  }

  const allowed =
    (permission ? auth.hasPermission(permission) : true) &&
    (anyPermissions?.length ? auth.hasAnyPermission(anyPermissions) : true) &&
    (allPermissions?.length ? auth.hasAllPermissions(allPermissions) : true);

  if (!allowed) {
    return <ForbiddenPage />;
  }
  return <Outlet />;
}

