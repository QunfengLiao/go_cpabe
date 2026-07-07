import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "./AuthContext";
import { AccountSwitchPage } from "../pages/AccountSwitchPage";

export function RequireAuth() {
  const auth = useAuth();
  const location = useLocation();

  if (!auth.isAuthenticated) {
    const hasSwitchableAccount = auth.cachedAccounts.some((account) => auth.hasAccountSession(account.userId));
    if (hasSwitchableAccount) {
      return <AccountSwitchPage />;
    }
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  return <Outlet />;
}
