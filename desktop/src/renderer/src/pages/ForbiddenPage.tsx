import { Button, Result } from "antd";
import { useNavigate } from "react-router-dom";

export function ForbiddenPage({ mode = "forbidden", message }: { mode?: "forbidden" | "error"; message?: string }) {
  const navigate = useNavigate();
  const isError = mode === "error";
  return (
    <Result
      className="forbidden-page"
      status={isError ? "warning" : "403"}
      title={isError ? "授权上下文不可用" : "无权访问"}
      subTitle={message ?? (isError ? "当前账号授权加载失败，请稍后重试或重新登录。" : "当前账号没有访问此页面所需的租户权限。")}
      extra={[
        <Button key="profile" type="primary" onClick={() => navigate("/profile", { replace: true })}>
          返回当前用户
        </Button>,
        <Button key="tenant" onClick={() => navigate("/select-tenant", { replace: true })}>
          切换租户
        </Button>
      ]}
    />
  );
}

