import { Link, Outlet } from "react-router-dom";
import { useAuth } from "./AuthContext";
import { Alert } from "../components/Alert";

export function RequirePlatformAdmin() {
  const auth = useAuth();

  if (!auth.isPlatformAdmin) {
    return (
      <section className="platform-page">
        <div className="platform-header">
          <div>
            <h2>无权访问平台后台</h2>
            <p>当前账号没有 PLATFORM_ADMIN 平台级角色，请使用平台管理员账号登录。</p>
          </div>
          <Link className="secondary-action" to="/profile">
            返回个人中心
          </Link>
        </div>
        <Alert type="error" message="当前账号没有平台管理权限" />
      </section>
    );
  }

  return <Outlet />;
}
