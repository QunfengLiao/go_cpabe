import { FormEvent, PointerEvent, useEffect, useRef, useState } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { login } from "../api/auth";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import { LoginRobot, type LoginRobotState } from "../components/LoginRobot";

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const isAddAccountMode = searchParams.get("mode") === "add-account";
  const cardRef = useRef<HTMLFormElement | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState((location.state as { notice?: string } | null)?.notice ?? auth.notice);
  const [loading, setLoading] = useState(false);
  const [focusedField, setFocusedField] = useState<"email" | "password" | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [loginSucceeded, setLoginSucceeded] = useState(false);
  const [eyeOffset, setEyeOffset] = useState({ x: 0, y: 0 });

  useEffect(() => {
    if (auth.notice) auth.consumeNotice();
  }, [auth]);

  if (auth.isAuthenticated && !isAddAccountMode) {
    return <Navigate to="/profile" replace />;
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setNotice("");
    setLoginSucceeded(false);
    if (!email.trim() || !password) {
      setError("请输入邮箱和密码");
      return;
    }
    setLoading(true);
    try {
      const data = await login({ email: email.trim(), password });
      setLoginSucceeded(true);
      await new Promise((resolve) => window.setTimeout(resolve, 450));
      auth.finishLogin(data);
      navigate("/profile", { replace: true });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "登录失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  function onCancelAddAccount() {
    // 添加账号只是临时进入登录表单，不能清空当前账号或 cachedAccounts，否则会破坏多账号切换上下文。
    if (window.history.length > 1) {
      navigate(-1);
      return;
    }
    navigate(auth.isAuthenticated ? "/profile" : "/login", { replace: true });
  }

  // 登录守卫动画只跟随表单交互状态变化，不参与认证逻辑，避免影响双 Token 登录流程。
  // 密码框聚焦时默认遮眼，是为了用视觉反馈提示“敏感凭证正在输入”，显示密码时再切换为半遮状态。
  const robotState: LoginRobotState = loginSucceeded
    ? "success"
    : error
      ? "error"
      : focusedField === "password" || showPassword
        ? showPassword
          ? "password-visible"
          : "password-focus"
        : focusedField === "email"
          ? "email-focus"
          : "normal";

  function onCardPointerMove(event: PointerEvent<HTMLFormElement>) {
    const head = cardRef.current?.querySelector<HTMLElement>(".login-robot-head");
    if (!head) return;
    const rect = head.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    const deltaX = event.clientX - centerX;
    const deltaY = event.clientY - centerY;
    const length = Math.hypot(deltaX, deltaY) || 1;
    const maxOffset = robotState === "password-focus" ? 2 : 7;

    // 眼球跟随只取方向和小幅位移，避免瞳孔跑出屏幕眼眶，保持安全产品的克制感。
    setEyeOffset({
      x: Math.round((deltaX / length) * maxOffset),
      y: Math.round((deltaY / length) * Math.min(maxOffset, 5))
    });
  }

  return (
    <form
      className="auth-card"
      onMouseLeave={() => setEyeOffset({ x: 0, y: 0 })}
      onPointerMove={onCardPointerMove}
      onSubmit={(event) => void onSubmit(event)}
      ref={cardRef}
    >
      {isAddAccountMode && (
        <button className="auth-back-button" onClick={onCancelAddAccount} type="button">
          ← 返回当前账号
        </button>
      )}
      <LoginRobot state={robotState} eyeOffset={eyeOffset} />
      <div className="form-title">
        <h2>{isAddAccountMode ? "添加账号" : "登录"}</h2>
        <p>{isAddAccountMode ? "登录另一个账号，原有账号记录会继续保留" : "进入你的加密文件共享工作台"}</p>
      </div>
      <Alert type="success" message={notice} />
      <Alert type="error" message={error} />
      <label className="field">
        <span>邮箱</span>
        <input
          value={email}
          onBlur={() => setFocusedField(null)}
          onChange={(event) => {
            setEmail(event.target.value);
            if (error) setError("");
          }}
          onFocus={() => setFocusedField("email")}
          type="email"
          autoComplete="email"
        />
      </label>
      <label className="field">
        <span>密码</span>
        <div className="password-control">
          <input
            value={password}
            onBlur={() => setFocusedField(null)}
            onChange={(event) => {
              setPassword(event.target.value);
              if (error) setError("");
            }}
            onFocus={() => setFocusedField("password")}
            type={showPassword ? "text" : "password"}
            autoComplete="current-password"
          />
          <button
            aria-label={showPassword ? "隐藏密码" : "显示密码"}
            className="password-toggle"
            onClick={() => setShowPassword((value) => !value)}
            onMouseDown={(event) => event.preventDefault()}
            type="button"
          >
            {showPassword ? "隐藏" : "显示"}
          </button>
        </div>
      </label>
      <button className="primary-action" type="submit" disabled={loading}>
        {loading ? "登录中..." : "登录"}
      </button>
      <p className="form-tip">
        还没有账号？<Link to="/register">创建新账号</Link>
      </p>
    </form>
  );
}
