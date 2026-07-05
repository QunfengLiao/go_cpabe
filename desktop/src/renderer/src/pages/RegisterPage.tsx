import { FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { register } from "../api/auth";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { UserRole } from "../types";

// CP-ABE 的核心差异在于“谁加密共享”和“谁按属性解密访问”，注册页直接用业务角色卡片帮助新手理解系统边界。
const roleOptions: Array<{
  value: Exclude<UserRole, "admin">;
  title: string;
  description: string;
}> = [
  {
    value: "data_owner",
    title: "数据拥有者",
    description: "上传文件、设置访问策略、加密共享数据。"
  },
  {
    value: "data_user",
    title: "数据访问者",
    description: "查看授权文件、申请密钥、解密访问数据。"
  }
];

export function RegisterPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [nickname, setNickname] = useState("");
  const [role, setRole] = useState<Exclude<UserRole, "admin">>("data_owner");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (!email.trim() || !/^\S+@\S+\.\S+$/.test(email)) {
      setError("请输入正确的邮箱");
      return;
    }
    if (!password) {
      setError("请输入密码");
      return;
    }
    if (password !== confirmPassword) {
      setError("两次密码不一致");
      return;
    }
    if (!nickname.trim()) {
      setError("请输入昵称");
      return;
    }
    setLoading(true);
    try {
      await register({
        email: email.trim(),
        password,
        confirm_password: confirmPassword,
        nickname: nickname.trim(),
        role
      });
      navigate("/login", { replace: true, state: { notice: "注册成功，请登录" } });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "注册失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  return (
    <form className="auth-card auth-card-register" onSubmit={(event) => void onSubmit(event)}>
      <div className="form-title">
        <h2>创建账号</h2>
        <p>选择你在加密共享流程中的角色</p>
      </div>
      <Alert type="error" message={error} />
      <label className="field">
        <span>邮箱</span>
        <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="email" />
      </label>
      <label className="field">
        <span>密码</span>
        <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="new-password" />
      </label>
      <label className="field">
        <span>确认密码</span>
        <input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" />
      </label>
      <label className="field">
        <span>昵称</span>
        <input value={nickname} onChange={(event) => setNickname(event.target.value)} maxLength={20} />
      </label>
      <div className="field">
        <span>角色</span>
        <div className="role-card-grid" role="radiogroup" aria-label="注册角色">
          {roleOptions.map((option) => (
            <button
              aria-checked={role === option.value}
              className={role === option.value ? "role-card role-card-active" : "role-card"}
              key={option.value}
              onClick={() => setRole(option.value)}
              role="radio"
              type="button"
            >
              <strong>{option.title}</strong>
              <small>{option.description}</small>
            </button>
          ))}
        </div>
      </div>
      <button className="primary-action" type="submit" disabled={loading}>
        {loading ? "注册中..." : "注册"}
      </button>
      <p className="form-tip">
        已有账号？<Link to="/login">返回登录</Link>
      </p>
    </form>
  );
}
