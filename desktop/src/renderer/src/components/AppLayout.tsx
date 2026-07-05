import { useEffect, useMemo, useRef, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { API_BASE_URL } from "../api/request";
import { avatarInitial, resolveAvatarURL } from "../utils/avatar";
import type { CachedAccount } from "../types";
import { ThemeSwitcher } from "./ThemeSwitcher";

const ACCOUNT_MENU_WIDTH = 320;
const ACCOUNT_MENU_GAP = 10;
const VIEWPORT_MARGIN = 12;

export function AppLayout() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [switchingId, setSwitchingId] = useState("");
  const [accountMessage, setAccountMessage] = useState("");
  const accountButtonRef = useRef<HTMLButtonElement | null>(null);
  const accountMenuRef = useRef<HTMLDivElement | null>(null);
  const [accountMenuStyle, setAccountMenuStyle] = useState({ left: VIEWPORT_MARGIN, top: VIEWPORT_MARGIN });
  const upcomingModules = ["文件管理", "访问策略", "密钥管理", "加密算法对比"];
  const avatarURL = resolveAvatarURL(auth.user?.avatar_url);
  const sortedAccounts = useMemo(() => [...auth.cachedAccounts].sort((left, right) => {
    if (left.userId === auth.currentUserId) return -1;
    if (right.userId === auth.currentUserId) return 1;
    return right.lastActiveAt - left.lastActiveAt;
  }), [auth.cachedAccounts, auth.currentUserId]);

  function updateAccountMenuPosition() {
    const button = accountButtonRef.current;
    if (!button) return;
    const rect = button.getBoundingClientRect();
    const menuHeight = accountMenuRef.current?.offsetHeight ?? 360;
    const maxLeft = Math.max(VIEWPORT_MARGIN, window.innerWidth - ACCOUNT_MENU_WIDTH - VIEWPORT_MARGIN);
    const preferRight = rect.right + ACCOUNT_MENU_GAP + ACCOUNT_MENU_WIDTH <= window.innerWidth - VIEWPORT_MARGIN;
    const rawLeft = preferRight ? rect.right + ACCOUNT_MENU_GAP : rect.left;
    const left = Math.min(Math.max(VIEWPORT_MARGIN, rawLeft), maxLeft);
    const topCandidate = rect.top - menuHeight - ACCOUNT_MENU_GAP;
    const bottomCandidate = rect.bottom + ACCOUNT_MENU_GAP;
    const hasRoomAbove = topCandidate >= VIEWPORT_MARGIN;
    const rawTop = hasRoomAbove ? topCandidate : bottomCandidate;
    const maxTop = Math.max(VIEWPORT_MARGIN, window.innerHeight - menuHeight - VIEWPORT_MARGIN);
    const top = Math.min(Math.max(VIEWPORT_MARGIN, rawTop), maxTop);
    setAccountMenuStyle({ left, top });
  }

  useEffect(() => {
    if (!accountMenuOpen) return;
    updateAccountMenuPosition();

    function onPointerDown(event: PointerEvent) {
      const target = event.target as Node;
      if (accountButtonRef.current?.contains(target) || accountMenuRef.current?.contains(target)) return;
      setAccountMenuOpen(false);
    }

    function onReposition() {
      updateAccountMenuPosition();
    }

    document.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("resize", onReposition);
    window.addEventListener("scroll", onReposition, true);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("resize", onReposition);
      window.removeEventListener("scroll", onReposition, true);
    };
  }, [accountMenuOpen]);

  useEffect(() => {
    if (!accountMenuOpen) return;
    updateAccountMenuPosition();
  }, [accountMenuOpen, sortedAccounts.length, accountMessage]);

  async function onSwitchAccount(accountId: string) {
    if (accountId === auth.currentUserId) return;
    setAccountMessage("");
    setSwitchingId(accountId);
    try {
      await auth.switchAccount(accountId);
      setAccountMenuOpen(false);
    } catch (err) {
      setAccountMessage(err instanceof ApiError ? err.message : "该账号登录已过期，请重新登录");
    } finally {
      setSwitchingId("");
    }
  }

  async function onRemoveAccount(accountId: string) {
    setAccountMessage("");
    await auth.removeAccount(accountId);
  }

  const accountMenu =
    accountMenuOpen &&
    // 账号菜单宽于侧边栏，放在 sidebar 内部会被布局宽度或 overflow 裁剪；Portal + fixed 让弹层脱离侧边栏裁剪边界。
    createPortal(
      <AccountMenu
        accounts={sortedAccounts}
        currentUserId={auth.currentUserId}
        menuRef={accountMenuRef}
        message={accountMessage}
        onAddAccount={() => {
          setAccountMenuOpen(false);
          navigate("/login?mode=add-account");
        }}
        onClose={() => setAccountMenuOpen(false)}
        onLogout={() => void auth.logout()}
        onRemoveAccount={(accountId) => void onRemoveAccount(accountId)}
        onSwitchAccount={(accountId) => void onSwitchAccount(accountId)}
        switchingId={switchingId}
        style={accountMenuStyle}
      />,
      document.body
    );

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="brand-mark">CP</div>
          <div>
            <strong>CP-ABE</strong>
            <span>加密文件共享</span>
          </div>
        </div>
        <nav className="sidebar-nav" aria-label="主导航">
          <NavLink to="/profile" className={({ isActive }) => (isActive ? "nav-item nav-item-active" : "nav-item")}>
            当前用户
          </NavLink>
          <div className="nav-section-title">后续模块</div>
          {/* 这些入口先作为产品路线预告展示，避免用户误以为认证模块就是完整系统边界。 */}
          {upcomingModules.map((name) => (
            <div className="nav-item nav-item-disabled" key={name} aria-disabled="true">
              <span>{name}</span>
              <em>即将支持</em>
            </div>
          ))}
          <button className="nav-item nav-logout" type="button" onClick={() => void auth.logout()}>
            退出当前账号
          </button>
        </nav>
        <div className="sidebar-footer">
          {import.meta.env.DEV && (
            <div className="sidebar-meta">
              <span>后端 API</span>
              <strong>{API_BASE_URL}</strong>
            </div>
          )}
          <div className="sidebar-account-area">
            <button className="sidebar-user" type="button" onClick={() => setAccountMenuOpen((open) => !open)} aria-expanded={accountMenuOpen} ref={accountButtonRef}>
              {/* 头像依赖全局 user 状态；资料页上传成功后同步 auth.setUser，侧边栏才能立即刷新。 */}
              {avatarURL ? (
                <img src={avatarURL} alt="当前用户头像" />
              ) : (
                <div className="sidebar-avatar-fallback">{avatarInitial(auth.user?.nickname, auth.user?.email)}</div>
              )}
              <div>
                <strong>{auth.user?.nickname ?? "未登录"}</strong>
                <span>{roleLabel(auth.user?.role)}</span>
              </div>
              <span className="account-chevron">{accountMenuOpen ? "收起" : "切换"}</span>
            </button>
            {accountMenu}
          </div>
        </div>
      </aside>
      <section className="workspace">
        <header className="topbar">
          <div className="topbar-title">
            <h1>欢迎使用 CP-ABE 加密文件共享系统</h1>
            <p>支持数据拥有者上传加密文件，数据访问者访问授权文件</p>
          </div>
          <div className="topbar-actions">
            <ThemeSwitcher compact />
            <button className="secondary-action" type="button" onClick={() => void auth.logout()}>
              退出当前账号
            </button>
          </div>
        </header>
        <main className="content">
          <Outlet />
        </main>
      </section>
    </div>
  );
}

function AccountMenu({
  accounts,
  currentUserId,
  menuRef,
  message,
  onAddAccount,
  onClose,
  onLogout,
  onRemoveAccount,
  onSwitchAccount,
  switchingId,
  style
}: {
  accounts: CachedAccount[];
  currentUserId: string;
  menuRef: RefObject<HTMLDivElement | null>;
  message: string;
  onAddAccount: () => void;
  onClose: () => void;
  onLogout: () => void;
  onRemoveAccount: (accountId: string) => void;
  onSwitchAccount: (accountId: string) => void;
  switchingId: string;
  style: { left: number; top: number };
}) {
  return (
    <div className="account-menu" ref={menuRef} style={{ left: style.left, top: style.top }}>
      <div className="account-menu-header">
        <div className="account-menu-title">账号切换</div>
        <button className="account-menu-close" onClick={onClose} type="button">
          收起
        </button>
      </div>
      {message && <div className="account-menu-alert">{message}</div>}
      <div className="account-list">
        {accounts.map((account) => {
          const accountAvatar = resolveAvatarURL(account.avatarUrl);
          const isCurrent = account.userId === currentUserId;
          const unavailable = !account.refreshToken || account.expired || account.loggedOut;
          return (
            <div className={`account-row${isCurrent ? " account-row-current" : ""}`} key={account.userId}>
              <button
                className="account-row-main"
                disabled={isCurrent || unavailable || switchingId === account.userId}
                onClick={() => onSwitchAccount(account.userId)}
                type="button"
              >
                {accountAvatar ? (
                  <img src={accountAvatar} alt={`${account.nickname}头像`} />
                ) : (
                  <div className="sidebar-avatar-fallback">{avatarInitial(account.nickname, account.email)}</div>
                )}
                <span>
                  <strong>{account.nickname}</strong>
                  <small>{switchingId === account.userId ? "切换中..." : isCurrent ? "当前账号" : unavailable ? "登录已过期" : roleLabel(account.role)}</small>
                </span>
              </button>
              <button className="account-remove" onClick={() => onRemoveAccount(account.userId)} type="button">
                移除
              </button>
            </div>
          );
        })}
      </div>
      <div className="account-menu-actions">
        <button className="secondary-action" type="button" onClick={onAddAccount}>
          添加账号
        </button>
        <button className="secondary-action danger-action" type="button" onClick={onLogout}>
          退出当前账号
        </button>
      </div>
    </div>
  );
}

function roleLabel(role?: string) {
  if (role === "data_owner") return "数据拥有者";
  if (role === "data_user") return "数据访问者";
  if (role === "admin") return "系统管理员";
  return "请先登录";
}
