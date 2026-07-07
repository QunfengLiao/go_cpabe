import { FormEvent, PointerEvent, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { login } from "../api/auth";
import { clearTenantStartupSession, getCurrentTenantCode, saveLastTenantCode } from "../api/authStorage";
import { getCredentialByEmail, getSavedEmails, removeCredential, saveCredential } from "../api/credentialStore";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import { LoginRobot, type LoginRobotState } from "../components/LoginRobot";
import { getTenantLoginConfig } from "../config/tenantLoginConfigs";
import type { CachedAccount, LoginData } from "../types";
import { avatarInitial, resolveAvatarURL } from "../utils/avatar";

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { tenantCode } = useParams();
  const tenantConfig = getTenantLoginConfig(tenantCode);
  const routeTenantCode = tenantCode && tenantConfig ? tenantCode : undefined;
  const invalidTenantCode = Boolean(tenantCode && !tenantConfig);
  const searchParams = new URLSearchParams(location.search);
  const isAddAccountMode = searchParams.get("mode") === "add-account";
  const cardRef = useRef<HTMLFormElement | null>(null);
  const credentialFieldRef = useRef<HTMLLabelElement | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState((location.state as { notice?: string } | null)?.notice ?? auth.notice);
  const [loading, setLoading] = useState(false);
  const [focusedField, setFocusedField] = useState<"email" | "password" | null>(null);
  const [showPassword, setShowPassword] = useState(false);
  const [loginSucceeded, setLoginSucceeded] = useState(false);
  const [eyeOffset, setEyeOffset] = useState({ x: 0, y: 0 });
  const [showPasswordLogin, setShowPasswordLogin] = useState(Boolean(routeTenantCode) || isAddAccountMode || auth.cachedAccounts.length === 0);
  const [switchingAccountId, setSwitchingAccountId] = useState("");
  const [savedCredentialEmails, setSavedCredentialEmails] = useState<string[]>([]);
  const [credentialSuggestionsOpen, setCredentialSuggestionsOpen] = useState(false);
  const [autoFilledEmail, setAutoFilledEmail] = useState("");
  const [pendingRemember, setPendingRemember] = useState<{ email: string; password: string; data: LoginData; mode: "save" | "update" } | null>(null);
  const savedAccounts = auth.cachedAccounts;
  const showAccountPicker = !isAddAccountMode && !showPasswordLogin && savedAccounts.length > 0;
  const filteredCredentialEmails = useMemo(() => {
    const keyword = email.trim().toLowerCase();
    return savedCredentialEmails.filter((item) => !keyword || item.includes(keyword));
  }, [email, savedCredentialEmails]);
  const showCredentialSuggestions = credentialSuggestionsOpen && filteredCredentialEmails.length > 0 && !showAccountPicker;

  useEffect(() => {
    if (auth.notice) auth.consumeNotice();
  }, [auth]);

  useEffect(() => {
    if (!routeTenantCode) return;
    const currentTenantCode = getCurrentTenantCode();
    if (currentTenantCode && currentTenantCode !== routeTenantCode) {
      // 租户登录入口代表明确的安全边界，不能沿用其他租户留下的 token 和用户上下文。
      clearTenantStartupSession();
      auth.clearAuth();
    }
    saveLastTenantCode(routeTenantCode);
  }, [auth.clearAuth, routeTenantCode]);

  useEffect(() => {
    if (routeTenantCode || isAddAccountMode || savedAccounts.length === 0) {
      setShowPasswordLogin(true);
    }
  }, [isAddAccountMode, routeTenantCode, savedAccounts.length]);

  useEffect(() => {
    let cancelled = false;
    void getSavedEmails()
      .then((emails) => {
        if (!cancelled) setSavedCredentialEmails(emails);
      })
      .catch(() => {
        if (!cancelled) setSavedCredentialEmails([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    function onPointerDown(event: globalThis.PointerEvent) {
      const target = event.target as Node;
      if (credentialFieldRef.current?.contains(target)) return;
      setCredentialSuggestionsOpen(false);
    }

    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, []);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setNotice("");
    setLoginSucceeded(false);
    const normalizedEmail = email.trim().toLowerCase();
    if (!normalizedEmail || !password) {
      setError("请输入邮箱和密码");
      return;
    }
    if (invalidTenantCode) {
      setError("租户不存在或暂未启用，请返回租户选择页");
      return;
    }
    setLoading(true);
    try {
      const data = await login({ email: normalizedEmail, password, tenantCode: routeTenantCode });
      setLoginSucceeded(true);
      const savedPassword = await getCredentialByEmail(normalizedEmail);
      if (savedPassword && savedPassword === password) {
        await finishSuccessfulLogin(data);
        return;
      }
      setPendingRemember({ email: normalizedEmail, password, data, mode: savedPassword ? "update" : "save" });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "登录失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  async function finishSuccessfulLogin(data: LoginData) {
    await new Promise((resolve) => window.setTimeout(resolve, 450));
    auth.finishLogin(data);
    const platformRoles = data.platform_roles ?? data.platformRoles ?? [];
    if (platformRoles.includes("PLATFORM_ADMIN")) {
      navigate("/platform", { replace: true });
      return;
    }
    if (!routeTenantCode && (data.tenants?.length ?? 0) > 1) {
      navigate("/select-tenant", { replace: true });
      return;
    }
    navigate("/profile", { replace: true });
  }

  async function onRememberChoice(remember: boolean) {
    if (!pendingRemember) return;
    const { email: loginEmail, password: loginPassword, data } = pendingRemember;
    setPendingRemember(null);

    try {
      if (remember) {
        await saveCredential(loginEmail, loginPassword);
        setSavedCredentialEmails(await getSavedEmails());
      } else {
        await removeCredential(loginEmail);
        setSavedCredentialEmails(await getSavedEmails());
      }
    } catch (err) {
      window.alert(err instanceof Error ? `密码未保存：${err.message}` : "密码未保存：当前系统安全存储不可用");
    }

    await finishSuccessfulLogin(data);
  }

  function onCancelAddAccount() {
    // “返回当前账号”是多账号流程的固定出口，不能依赖浏览器历史栈；
    // 用户可能刚从注册页回到添加账号登录页，历史后退会把他重新带回创建账号页。
    navigate(auth.isAuthenticated ? "/profile" : "/login", { replace: true });
  }

  function openPasswordLogin(account?: CachedAccount, message?: string) {
    setShowPasswordLogin(true);
    setPassword("");
    setShowPassword(false);
    setLoginSucceeded(false);
    if (account) setEmail(account.email);
    if (message) {
      setNotice("");
      setError(message);
    } else {
      setError("");
    }
  }

  async function onQuickLogin(account: CachedAccount) {
    setError("");
    setNotice("");
    setLoginSucceeded(false);

    if (!auth.hasAccountSession(account.userId)) {
      openPasswordLogin(account, "该账号需要重新输入密码后才能登录");
      return;
    }

    setSwitchingAccountId(account.userId);
    try {
      await auth.switchAccount(account.userId);
      setLoginSucceeded(true);
    } catch (err) {
      openPasswordLogin(account, err instanceof ApiError ? "该账号登录已过期，请输入密码重新登录" : "登录态恢复失败，请输入密码重新登录");
    } finally {
      setSwitchingAccountId("");
    }
  }

  async function onRemoveAccount(account: CachedAccount) {
    if (switchingAccountId) return;
    await auth.removeAccount(account.userId);
    if (email === account.email) {
      setEmail("");
      setPassword("");
    }
  }

  function onUseOtherAccount() {
    setEmail("");
    setPassword("");
    setNotice("");
    setError("");
    setAutoFilledEmail("");
    setShowPasswordLogin(true);
  }

  function onBackToAccountPicker() {
    setNotice("");
    setError("");
    setPassword("");
    setAutoFilledEmail("");
    setCredentialSuggestionsOpen(false);
    setShowPasswordLogin(false);
  }

  function onEmailChange(value: string) {
    const normalizedEmail = value.trim().toLowerCase();
    setEmail(value);
    setCredentialSuggestionsOpen(true);
    if (!normalizedEmail || (autoFilledEmail && normalizedEmail !== autoFilledEmail)) {
      setPassword("");
      setAutoFilledEmail("");
    }
    if (error) setError("");
  }

  async function onSelectCredentialEmail(nextEmail: string) {
    const normalizedEmail = nextEmail.trim().toLowerCase();
    setEmail(normalizedEmail);
    setCredentialSuggestionsOpen(false);
    setError("");
    try {
      const savedPassword = await getCredentialByEmail(normalizedEmail);
      setPassword(savedPassword);
      setAutoFilledEmail(savedPassword ? normalizedEmail : "");
    } catch (err) {
      setPassword("");
      setAutoFilledEmail("");
      setError(err instanceof Error ? err.message : "读取本地保存密码失败");
    }
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

  const loginDescription = invalidTenantCode
    ? "当前租户入口不可用"
    : tenantConfig?.code
      ? tenantConfig.subtitle
      : showAccountPicker
        ? "选择本机登录过的账号，系统会尝试恢复登录态"
        : isAddAccountMode
          ? "登录另一个账号，原有账号记录会继续保留"
          : "进入你的加密文件共享工作台";

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
        <h2>{showAccountPicker ? "欢迎回来" : isAddAccountMode ? "添加账号" : "登录"}</h2>
        <p>{loginDescription}</p>
      </div>
      <Alert type="success" message={notice} />
      <Alert type="error" message={invalidTenantCode ? "租户不存在或暂未启用，请检查登录链接或重新选择租户。" : ""} />
      <Alert type="error" message={error} />

      {showAccountPicker ? (
        <>
          <div className="login-account-list">
            {savedAccounts.map((account) => {
              const avatarURL = resolveAvatarURL(account.avatarUrl);
              const canQuickLogin = auth.hasAccountSession(account.userId);
              const switching = switchingAccountId === account.userId;
              return (
                <div className="login-account-row" key={account.userId}>
                  <button className="login-account-main" disabled={Boolean(switchingAccountId)} onClick={() => void onQuickLogin(account)} type="button">
                    {avatarURL ? (
                      <img className="login-account-avatar" src={avatarURL} alt={`${account.nickname}头像`} />
                    ) : (
                      <div className="login-account-avatar login-account-avatar-fallback">{avatarInitial(account.nickname, account.email)}</div>
                    )}
                    <span className="login-account-info">
                      <strong>{account.nickname}</strong>
                      <small>{account.email}</small>
                      <small className="login-account-time">上次登录：{formatLastLoginAt(account.lastLoginAt)}</small>
                    </span>
                    <em className={canQuickLogin ? "login-account-status" : "login-account-status login-account-status-muted"}>
                      {switching ? "恢复中..." : canQuickLogin ? "快捷登录" : "需密码"}
                    </em>
                  </button>
                  <button
                    aria-label={`从此设备移除 ${account.email}`}
                    className="login-account-remove"
                    disabled={Boolean(switchingAccountId)}
                    onClick={() => void onRemoveAccount(account)}
                    type="button"
                  >
                    移除
                  </button>
                </div>
              );
            })}
          </div>
          <button className="secondary-action auth-wide-action" onClick={onUseOtherAccount} type="button">
            使用其他账号登录
          </button>
          <Link className="tenant-select-link" to="/select-tenant">
            切换租户入口
          </Link>
          <p className="form-tip">本机只保存账号展示信息，不保存密码。</p>
        </>
      ) : (
        <>
          {!routeTenantCode && !isAddAccountMode && savedAccounts.length > 0 && (
            <button className="auth-back-button" onClick={onBackToAccountPicker} type="button">
              ← 返回账号列表
            </button>
          )}
          <label className="field credential-field" ref={credentialFieldRef}>
            <span>邮箱</span>
            <input
              value={email}
              onBlur={() => setFocusedField(null)}
              onChange={(event) => onEmailChange(event.target.value)}
              onFocus={() => {
                setFocusedField("email");
                if (savedCredentialEmails.length > 0) setCredentialSuggestionsOpen(true);
              }}
              type="email"
              autoComplete="email"
            />
            {showCredentialSuggestions && (
              <div className="credential-suggestions" role="listbox" aria-label="历史登录邮箱">
                {filteredCredentialEmails.map((item) => (
                  <button key={item} type="button" onClick={() => void onSelectCredentialEmail(item)}>
                    {item}
                  </button>
                ))}
              </div>
            )}
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
          <button className="primary-action" type="submit" disabled={loading || invalidTenantCode}>
            {loading ? "登录中..." : tenantConfig?.buttonText ?? "登录"}
          </button>
          <p className="form-tip">
            还没有账号？<Link to={isAddAccountMode ? "/register?mode=add-account" : "/register"}>创建新账号</Link>
          </p>
          <Link className="tenant-select-link" to="/select-tenant">
            选择其他租户入口
          </Link>
        </>
      )}
      {pendingRemember &&
        createPortal(
          <div className="remember-modal-backdrop" role="presentation">
            <div className="remember-modal" role="dialog" aria-modal="true" aria-labelledby="remember-password-title">
            <h3 id="remember-password-title">{pendingRemember.mode === "update" ? "是否更新保存的密码？" : "是否记住密码？"}</h3>
            <p>
              {pendingRemember.mode === "update"
                ? "该邮箱已保存过密码。本次密码不同，更新后下次选择该邮箱会自动填充新密码。"
                : "记住后，下次登录时选择该邮箱可自动填充密码。"}
            </p>
              <div className="remember-modal-actions">
                <button className="primary-action" type="button" onClick={() => void onRememberChoice(true)}>
                  记住
                </button>
                <button className="secondary-action" type="button" onClick={() => void onRememberChoice(false)}>
                  不记住
                </button>
              </div>
            </div>
          </div>,
          document.body
        )}
    </form>
  );
}

function formatLastLoginAt(value: number): string {
  if (!value) return "暂无记录";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "暂无记录";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}
