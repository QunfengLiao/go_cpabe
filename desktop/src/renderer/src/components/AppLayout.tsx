import { useEffect, useMemo, useRef, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { AnimatePresence, motion } from "framer-motion";
import { Button, Layout, Menu, Tooltip, type MenuProps } from "antd";
import {
  ApartmentOutlined,
  DashboardOutlined,
  DownOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  RightOutlined,
  SafetyCertificateOutlined,
  UserOutlined
} from "@ant-design/icons";
import { ApiError } from "../api/request";
import { useAuth } from "../auth/AuthContext";
import { API_BASE_URL } from "../api/request";
import { avatarInitial, resolveAvatarURL } from "../utils/avatar";
import type { CachedAccount, TenantRole } from "../types";
import { ThemeSwitcher } from "./ThemeSwitcher";
import { brandingForTenant } from "../theme/tenantBranding";

const { Sider, Content } = Layout;
const ACCOUNT_MENU_WIDTH = 320;
const ACCOUNT_MENU_GAP = 10;
const VIEWPORT_MARGIN = 12;

export function AppLayout() {
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => window.localStorage.getItem("sidebar-collapsed") === "true");
  const [openKeys, setOpenKeys] = useState<string[]>(() => {
    const stored = window.localStorage.getItem("sidebar-open-keys");
    return stored ? JSON.parse(stored) as string[] : ["platform", "policy", "tenant"];
  });
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [switchingId, setSwitchingId] = useState("");
  const [accountMessage, setAccountMessage] = useState("");
  const [tenantLogoFailed, setTenantLogoFailed] = useState(false);
  const accountButtonRef = useRef<HTMLButtonElement | null>(null);
  const accountMenuRef = useRef<HTMLDivElement | null>(null);
  const [accountMenuStyle, setAccountMenuStyle] = useState({ left: VIEWPORT_MARGIN, top: VIEWPORT_MARGIN });
  const avatarURL = resolveAvatarURL(auth.user?.avatar_url);
  const currentTenant = auth.currentTenant ?? auth.tenants.find((tenant) => String(tenant.tenant_id) === auth.currentTenantId);
  const tenantBranding = brandingForTenant(currentTenant, auth.isPlatformAdmin);
  const tenantLogo = tenantLogoFailed ? undefined : tenantBranding.logoUrl;
  const tenantDisplayName = currentTenant?.tenant_name ?? (auth.isPlatformAdmin ? "平台管理" : "未选择租户");
  const tenantDisplayCode = currentTenant?.tenant_code ?? currentTenant?.tenantCode ?? (auth.isPlatformAdmin ? "PLATFORM" : "TENANT");
  const currentRoleLabel = roleLabel(auth.user?.role, auth.tenantRoles);
  const sortedAccounts = useMemo(() => [...auth.cachedAccounts].sort((left, right) => {
    if (left.userId === auth.currentUserId) return -1;
    if (right.userId === auth.currentUserId) return 1;
    return right.lastLoginAt - left.lastLoginAt;
  }), [auth.cachedAccounts, auth.currentUserId]);

  useEffect(() => {
    window.localStorage.setItem("sidebar-collapsed", String(sidebarCollapsed));
  }, [sidebarCollapsed]);

  useEffect(() => {
    window.localStorage.setItem("sidebar-open-keys", JSON.stringify(openKeys));
  }, [openKeys]);

  useEffect(() => {
    setTenantLogoFailed(false);
  }, [tenantBranding.logoUrl]);

  const selectedMenuKey = selectedKeyForPath(location.pathname);
  const menuItems = useMemo<MenuProps["items"]>(() => {
    const items: MenuProps["items"] = [
      { key: "/profile", icon: <UserOutlined />, label: "当前用户" }
    ];
    if (auth.isPlatformAdmin) {
      items.push({
        key: "platform",
        icon: <SafetyCertificateOutlined />,
        label: "平台管理",
        children: [
          { key: "/platform", icon: <DashboardOutlined />, label: "平台控制台" },
          { key: "/platform/tenants", icon: <ApartmentOutlined />, label: "租户列表" },
          { key: "/platform/policies", icon: <SafetyCertificateOutlined />, label: "访问策略管理" }
        ]
      });
    }
    const policyChildren = [
      auth.hasPermission("policy.write") ? { key: "/access-policies/builder", icon: <SafetyCertificateOutlined />, label: "访问策略构建" } : null,
      auth.hasPermission("policy.read") ? { key: "/access-policies", icon: <SafetyCertificateOutlined />, label: "我的访问策略" } : null
    ].filter(Boolean) as NonNullable<MenuProps["items"]>;
    if (policyChildren.length > 0) {
      items.push({
        key: "policy",
        icon: <SafetyCertificateOutlined />,
        label: "访问策略",
        children: policyChildren
      });
    }
    const tenantChildren = [
      auth.hasPermission("tenant.role.read") ? { key: "/tenant/roles", icon: <SafetyCertificateOutlined />, label: "角色管理" } : null,
      auth.hasPermission("tenant.member.read") ? { key: "/tenant/members", icon: <UserOutlined />, label: "成员角色" } : null,
      auth.hasPermission("tenant.org.read") ? { key: "/tenant/org-management", icon: <ApartmentOutlined />, label: "组织管理" } : null,
      auth.hasPermission("policy.read") ? { key: "/tenant/access-policies", icon: <SafetyCertificateOutlined />, label: "访问策略" } : null
    ].filter(Boolean) as NonNullable<MenuProps["items"]>;
    if (tenantChildren.length > 0) {
      items.push({
        key: "tenant",
        icon: <ApartmentOutlined />,
        label: "租户管理",
        children: tenantChildren
      });
    }
    return items;
  }, [auth]);

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

  function onMenuClick({ key }: { key: string }) {
    navigate(key);
  }

  const accountMenu =
    accountMenuOpen &&
    createPortal(
      <AccountMenu
        accounts={sortedAccounts}
        currentUserId={auth.currentUserId}
        hasAccountSession={auth.hasAccountSession}
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
    <Layout className="app-shell antd-app-shell">
      <Sider
        className="antd-sidebar"
        width={248}
        collapsedWidth={72}
        collapsed={sidebarCollapsed}
        trigger={null}
      >
        <div className="sidebar-brand">
          <div className="sidebar-brand-main">
            {tenantLogo ? (
              <img
                className="brand-mark brand-mark-image"
                src={tenantLogo}
                alt={`${tenantDisplayName} Logo`}
                onError={() => setTenantLogoFailed(true)}
              />
            ) : (
              <div className="brand-mark">{tenantDisplayCode.slice(0, 2).toUpperCase()}</div>
            )}
            {!sidebarCollapsed && (
              <div className="sidebar-brand-copy">
                <strong>{tenantDisplayName}</strong>
                <span>{tenantDisplayCode} · CP-ABE</span>
              </div>
            )}
          </div>
          <Tooltip title={sidebarCollapsed ? "展开侧边栏" : "折叠侧边栏"} placement="right">
            <Button
              className="sidebar-collapse-button"
              icon={sidebarCollapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              shape="circle"
              size="small"
              onClick={() => setSidebarCollapsed((collapsed) => !collapsed)}
              aria-label={sidebarCollapsed ? "展开侧边栏" : "折叠侧边栏"}
            />
          </Tooltip>
        </div>
        <Menu
          className="antd-sidebar-menu"
          mode="inline"
          inlineCollapsed={sidebarCollapsed}
          items={menuItems}
          selectedKeys={[selectedMenuKey]}
          openKeys={sidebarCollapsed ? [] : openKeys}
          onOpenChange={setOpenKeys}
          onClick={onMenuClick}
          expandIcon={({ isOpen }) => isOpen ? <DownOutlined className="menu-expand-icon menu-expand-icon-open" /> : <RightOutlined className="menu-expand-icon" />}
        />
        <div className="sidebar-footer">
          <div className="sidebar-tools">
            {!sidebarCollapsed && <ThemeSwitcher compact />}
          </div>
          {import.meta.env.DEV && !sidebarCollapsed && (
            <div className="sidebar-meta">
              <span>后端 API</span>
              <strong>{API_BASE_URL}</strong>
            </div>
          )}
          <div className="sidebar-account-area">
            <Tooltip title={sidebarCollapsed ? auth.user?.nickname ?? "未登录" : ""} placement="right">
              <button className="sidebar-user" type="button" onClick={() => setAccountMenuOpen((open) => !open)} aria-expanded={accountMenuOpen} ref={accountButtonRef}>
                {avatarURL ? (
                  <img src={avatarURL} alt="当前用户头像" />
                ) : (
                  <div className="sidebar-avatar-fallback">{avatarInitial(auth.user?.nickname, auth.user?.email)}</div>
                )}
                {!sidebarCollapsed && (
                  <>
                    <div className="sidebar-user-copy">
                      <strong>{auth.user?.nickname ?? "未登录"}</strong>
                      <span>{currentRoleLabel}</span>
                    </div>
                    <span className="account-chevron">{accountMenuOpen ? "收起" : "切换"}</span>
                  </>
                )}
              </button>
            </Tooltip>
            {accountMenu}
          </div>
        </div>
      </Sider>
      <Layout className="workspace">
        <div className="workspace-brand-layer" aria-hidden="true" />
        <Content className="content">
          {auth.user?.must_change_password && (
            <div className="password-change-banner">
              <div>
                <span>当前账号仍在使用初始密码，为保障安全，请尽快修改。</span>
              </div>
              <Button size="small" onClick={() => navigate("/profile")}>
                修改密码
              </Button>
            </div>
          )}
          <AnimatePresence mode="wait">
            <motion.div
              className="route-page-frame"
              key={location.pathname}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.18, ease: "easeOut" }}
            >
              <Outlet />
            </motion.div>
          </AnimatePresence>
        </Content>
      </Layout>
    </Layout>
  );
}

function selectedKeyForPath(pathname: string) {
  if (pathname === "/platform") return "/platform";
  if (pathname.startsWith("/platform/tenants")) return "/platform/tenants";
  if (pathname.startsWith("/platform/policies")) return "/platform/policies";
  if (pathname.startsWith("/access-policies/builder")) return "/access-policies/builder";
  if (pathname.startsWith("/access-policies")) return "/access-policies";
  if (pathname.startsWith("/tenant/roles")) return "/tenant/roles";
  if (pathname.startsWith("/tenant/members")) return "/tenant/members";
  if (pathname.startsWith("/tenant/org-management")) return "/tenant/org-management";
  if (pathname.startsWith("/tenant/access-policies")) return "/tenant/access-policies";
  return pathname;
}

function AccountMenu({
  accounts,
  currentUserId,
  hasAccountSession,
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
  hasAccountSession: (accountId: string) => boolean;
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
        <Button size="small" onClick={onClose} type="text">
          收起
        </Button>
      </div>
      {message && <div className="account-menu-alert">{message}</div>}
      <div className="account-list">
        {accounts.map((account) => {
          const accountAvatar = resolveAvatarURL(account.avatarUrl);
          const isCurrent = account.userId === currentUserId;
          const unavailable = !hasAccountSession(account.userId);
          const statusText = account.status === "login_required" ? "需要重新登录" : unavailable ? "登录已过期" : roleLabel(account.role);
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
                  <small>{switchingId === account.userId ? "切换中..." : isCurrent ? "当前账号" : statusText}</small>
                </span>
              </button>
              <Button size="small" type="text" danger onClick={() => onRemoveAccount(account.userId)}>
                移除
              </Button>
            </div>
          );
        })}
      </div>
      <div className="account-menu-actions">
        <Button onClick={onAddAccount}>添加账号</Button>
        <Button danger onClick={onLogout}>退出当前账号</Button>
      </div>
    </div>
  );
}

function roleLabel(role?: string, tenantRoles?: TenantRole[]) {
  const tenantLabels = sortTenantRolesForDisplay(tenantRoles ?? []).map(tenantRoleLabel).filter(Boolean);
  if (tenantLabels.length > 0) return tenantLabels.join(" / ");
  if (role === "data_owner") return "数据拥有者";
  if (role === "data_user") return "数据访问者";
  if (role === "admin") return "系统管理员";
  return "请先登录";
}

function tenantRoleLabel(role: TenantRole) {
  if (role === "TENANT_ADMIN") return "租户管理员";
  if (role === "DO") return "数据拥有者";
  if (role === "DU") return "数据访问者";
  if (role === "PLATFORM_ADMIN") return "平台管理员";
  return String(role || "");
}

function sortTenantRolesForDisplay(roles: TenantRole[]) {
  const priority: Record<string, number> = {
    TENANT_ADMIN: 0,
    DO: 1,
    DU: 2,
    PLATFORM_ADMIN: 9
  };
  return [...roles].sort((left, right) => (priority[left] ?? 5) - (priority[right] ?? 5));
}
