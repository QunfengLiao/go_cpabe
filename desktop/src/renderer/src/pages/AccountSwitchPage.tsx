import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { Alert } from "../components/Alert";
import { avatarInitial, resolveAvatarURL } from "../utils/avatar";

export function AccountSwitchPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState(auth.notice);
  const [switchingId, setSwitchingId] = useState("");
  const accounts = auth.cachedAccounts;
  const switchableAccounts = accounts.filter((account) => account.refreshToken && !account.expired && !account.loggedOut);

  async function onSwitch(accountId: string) {
    setError("");
    setSwitchingId(accountId);
    try {
      await auth.switchAccount(accountId);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "该账号登录已过期，请重新登录");
    } finally {
      setSwitchingId("");
    }
  }

  return (
    <section className="account-switch-page">
      <div className="account-switch-card panel">
        <div className="form-title">
          <h2>选择账号</h2>
          <p>选择本地保存过的账号恢复登录，无需重新输入密码</p>
        </div>
        <Alert type="error" message={error} />
        {switchableAccounts.length > 0 ? (
          <div className="account-switch-list">
            {accounts.map((account) => {
              const avatarURL = resolveAvatarURL(account.avatarUrl);
              const unavailable = !account.refreshToken || account.expired || account.loggedOut;
              return (
                <button
                  className="account-switch-item"
                  disabled={unavailable || switchingId === account.userId}
                  key={account.userId}
                  onClick={() => void onSwitch(account.userId)}
                  type="button"
                >
                  {avatarURL ? <img src={avatarURL} alt={`${account.nickname}头像`} /> : <div className="sidebar-avatar-fallback">{avatarInitial(account.nickname, account.email)}</div>}
                  <span>
                    <strong>{account.nickname}</strong>
                    <small>{unavailable ? "登录已过期，请重新登录" : account.email}</small>
                  </span>
                  <em>{switchingId === account.userId ? "切换中..." : unavailable ? "已过期" : "切换"}</em>
                </button>
              );
            })}
          </div>
        ) : (
          <div className="empty-state">没有可直接切换的账号，请重新登录。</div>
        )}
        <div className="account-switch-actions">
          <button className="primary-action" type="button" onClick={() => navigate("/login?mode=add-account")}>
            添加账号
          </button>
          <Link to="/login">使用密码登录</Link>
        </div>
      </div>
    </section>
  );
}
