import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "./AuthContext";

export function GuestOnly() {
  const auth = useAuth();
  const location = useLocation();
  const isAddAccountMode = new URLSearchParams(location.search).get("mode") === "add-account";

  // 已登录用户继续停留在登录/注册页会造成状态歧义：本地已有 Token，却还能看到创建账号入口。
  // 因此游客页只服务未登录用户，登录后统一回到当前用户资料页作为临时首页。
  if (auth.isAuthenticated && !isAddAccountMode) {
    return <Navigate to="/profile" replace />;
  }

  return <Outlet />;
}
