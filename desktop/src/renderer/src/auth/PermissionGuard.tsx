import { cloneElement, isValidElement, type ReactElement, type ReactNode } from "react";
import { useAuth } from "./AuthContext";

interface PermissionGuardProps {
  permission?: string;
  anyPermissions?: string[];
  allPermissions?: string[];
  mode?: "hide" | "disable" | "fallback";
  fallback?: ReactNode;
  children: ReactNode;
}

export function PermissionGuard({ permission, anyPermissions, allPermissions, mode = "hide", fallback = null, children }: PermissionGuardProps) {
  const auth = useAuth();
  const allowed =
    (permission ? auth.hasPermission(permission) : true) &&
    (anyPermissions?.length ? auth.hasAnyPermission(anyPermissions) : true) &&
    (allPermissions?.length ? auth.hasAllPermissions(allPermissions) : true);

  if (allowed) return <>{children}</>;
  if (mode === "fallback") return <>{fallback}</>;
  if (mode === "disable") return <>{disableChildren(children)}</>;
  return null;
}

function disableChildren(children: ReactNode): ReactNode {
  if (!isValidElement(children)) {
    return children;
  }
  const element = children as ReactElement<Record<string, unknown>>;
  return cloneElement(element, {
    disabled: true,
    "aria-disabled": true
  });
}

