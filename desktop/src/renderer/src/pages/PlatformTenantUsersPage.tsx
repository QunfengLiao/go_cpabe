import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { Button, Drawer, Dropdown, Modal, Radio, Table, Tag, type TableColumnsType } from "antd";
import { ArrowLeftOutlined, MoreOutlined, ReloadOutlined, UserAddOutlined } from "@ant-design/icons";
import {
  addPlatformTenantUser,
  assignPlatformTenantAdmin,
  createPlatformTenantAdminAccount,
  getPlatformTenant,
  listPlatformTenantUsers,
  removePlatformTenantAdmin,
  removePlatformTenantUser,
  searchPlatformUsers
} from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { TenantMember, TenantSummary, User } from "../types";
import { TenantStatusBadge } from "./PlatformTenantListPage";

type ActiveDialog = "add-existing" | "create-admin" | null;
type PasswordMode = "generated" | "custom";

export function PlatformTenantUsersPage() {
  const { tenantId } = useParams();
  const numericTenantId = Number(tenantId);
  const [tenant, setTenant] = useState<TenantSummary | null>(null);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [activeDialog, setActiveDialog] = useState<ActiveDialog>(null);
  const [adminForm, setAdminForm] = useState({ username: "", displayName: "", email: "", phone: "", password: "" });
  const [passwordMode, setPasswordMode] = useState<PasswordMode>("generated");
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<User[]>([]);
  const [hasSearched, setHasSearched] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [workingKey, setWorkingKey] = useState("");

  async function loadUsers() {
    if (!numericTenantId) {
      setError("租户 ID 不合法");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const [tenantData, userData] = await Promise.all([getPlatformTenant(numericTenantId), listPlatformTenantUsers(numericTenantId)]);
      setTenant(tenantData);
      setMembers(userData);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取租户用户失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadUsers();
  }, [numericTenantId]);

  const activeAdminCount = useMemo(
    () => members.filter((member) => member.member_status === "active" && member.roles.includes("TENANT_ADMIN")).length,
    [members]
  );

  const memberIds = useMemo(() => new Set(members.map((member) => member.user_id)), [members]);

  const columns = useMemo<TableColumnsType<TenantMember>>(() => [
    {
      title: "用户",
      dataIndex: "nickname",
      render: (_, member) => (
        <span className="table-title-cell">
          <strong>{member.nickname || member.username || "未命名用户"}</strong>
          <small>{[member.username, member.phone].filter(Boolean).join(" · ") || `用户 ID：${member.user_id}`}</small>
        </span>
      )
    },
    { title: "邮箱", dataIndex: "email", ellipsis: true, render: (value?: string) => <span className="table-ellipsis" title={value || "-"}>{value || "-"}</span> },
    { title: "状态", dataIndex: "member_status", width: 104, render: (value: string) => <MemberStatusBadge status={value} /> },
    { title: "租户管理员", key: "tenant_admin", width: 124, render: (_, member) => <TenantAdminBadge enabled={member.roles.includes("TENANT_ADMIN")} /> },
    {
      title: "操作",
      key: "actions",
      width: 168,
      render: (_, member) => {
        const isAdmin = member.roles.includes("TENANT_ADMIN");
        const removeDisabled = Boolean(workingKey) || (isAdmin && activeAdminCount <= 1);
        return (
          <div className="inline-action-row row-action-cell">
            {isAdmin ? (
              <Button type="link" size="small" className="row-primary-action" onClick={() => void onRemoveAdmin(member)} disabled={Boolean(workingKey) || activeAdminCount <= 1}>
                {workingKey === `remove-admin-${member.user_id}` ? "取消中..." : "取消管理员"}
              </Button>
            ) : (
              <Button type="link" size="small" className="row-primary-action" onClick={() => void onAssignAdmin(member)} disabled={Boolean(workingKey) || member.member_status !== "active" || tenant?.status === "disabled"}>
                {workingKey === `assign-admin-${member.user_id}` ? "设置中..." : "设为管理员"}
              </Button>
            )}
            <Dropdown
              menu={{
                items: [{ key: "remove", label: "移出租户", danger: true, disabled: removeDisabled }],
                onClick: ({ key }) => {
                  if (key === "remove") confirmRemoveUser(member);
                }
              }}
              trigger={["click"]}
            >
              <Button type="text" size="small" icon={<MoreOutlined />} aria-label="更多操作" disabled={Boolean(workingKey)} />
            </Dropdown>
          </div>
        );
      }
    }
  ], [activeAdminCount, tenant?.status, workingKey]);

  async function onSearchExistingUser(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");
    const query = searchQuery.trim();
    if (!query) {
      setError("请输入用户名、邮箱或手机号");
      return;
    }
    setSearching(true);
    setHasSearched(true);
    try {
      setSearchResults(await searchPlatformUsers(query));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "搜索用户失败");
    } finally {
      setSearching(false);
    }
  }

  async function onAddExistingUser(user: User) {
    await runMemberAction(`add-user-${user.id}`, "用户已加入租户", async () => {
      await addPlatformTenantUser(numericTenantId, user.id);
      setActiveDialog(null);
      setSearchQuery("");
      setSearchResults([]);
      setHasSearched(false);
    });
  }

  async function onCreateTenantAdmin(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");
    if (!adminForm.username.trim() || !adminForm.displayName.trim() || !adminForm.email.trim()) {
      setError("请填写用户名、姓名和邮箱");
      return;
    }
    if (passwordMode === "custom" && !adminForm.password.trim()) {
      setError("请输入自定义初始密码");
      return;
    }
    setWorkingKey("create-admin");
    const nextPassword = passwordMode === "custom" ? adminForm.password.trim() : "";
    try {
      const result = await createPlatformTenantAdminAccount(numericTenantId, {
        username: adminForm.username.trim(),
        displayName: adminForm.displayName.trim(),
        email: adminForm.email.trim(),
        phone: adminForm.phone.trim() || undefined,
        password: nextPassword || undefined
      });
      setAdminForm({ username: "", displayName: "", email: "", phone: "", password: "" });
      setPasswordMode("generated");
      setActiveDialog(null);
      setSuccess(result.temporary_password ? `租户管理员账号已创建，默认初始密码：${result.temporary_password}` : "租户管理员账号已创建，首次登录后需要重置密码");
      await loadUsers();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "创建租户管理员失败");
    } finally {
      setWorkingKey("");
    }
  }

  function confirmRemoveUser(member: TenantMember) {
    const userLabel = member.nickname || member.email || member.username || `用户 ${member.user_id}`;
    Modal.confirm({
      title: "移出租户",
      content: `确认将 ${userLabel} 移出该租户？该操作不会删除用户账号。`,
      okText: "移出租户",
      cancelText: "取消",
      okButtonProps: { danger: true },
      onOk: () => onRemoveUser(member)
    });
  }

  async function onRemoveUser(member: TenantMember) {
    await runMemberAction(`remove-user-${member.user_id}`, "用户已移出租户", () => removePlatformTenantUser(numericTenantId, member.user_id));
  }

  async function onAssignAdmin(member: TenantMember) {
    await runMemberAction(`assign-admin-${member.user_id}`, "Tenant Admin 已分配", () => assignPlatformTenantAdmin(numericTenantId, member.user_id));
  }

  async function onRemoveAdmin(member: TenantMember) {
    if (activeAdminCount <= 1) {
      setError("不能取消最后一个有效 Tenant Admin");
      return;
    }
    await runMemberAction(`remove-admin-${member.user_id}`, "Tenant Admin 已取消", () => removePlatformTenantAdmin(numericTenantId, member.user_id));
  }

  async function runMemberAction(key: string, message: string, action: () => Promise<unknown>) {
    setError("");
    setSuccess("");
    setWorkingKey(key);
    try {
      await action();
      setSuccess(message);
      await loadUsers();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    } finally {
      setWorkingKey("");
    }
  }

  function closeDialog() {
    if (workingKey || searching) return;
    setActiveDialog(null);
    setSearchQuery("");
    setSearchResults([]);
    setHasSearched(false);
    setError("");
  }

  function closeCreateAdminDrawer() {
    if (workingKey === "create-admin") return;
    setActiveDialog(null);
    setAdminForm({ username: "", displayName: "", email: "", phone: "", password: "" });
    setPasswordMode("generated");
    setError("");
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div className="platform-title-row">
          <Link className="page-back-link" to={tenant ? `/platform/tenants/${tenant.tenant_id}` : "/platform/tenants"}>
            <Button icon={<ArrowLeftOutlined />} shape="circle" size="large" aria-label="返回详情" />
          </Link>
          <div>
            <h2>租户管理员设置</h2>
            <p>平台管理员仅负责为当前租户添加成员并指定 Tenant Admin。租户内组织架构、数据拥有者、数据访问者由 Tenant Admin 维护。</p>
          </div>
        </div>
        <div className="platform-actions">
          <Button type="primary" icon={<UserAddOutlined />} onClick={() => setActiveDialog("add-existing")} disabled={tenant?.status === "disabled"}>
            添加已有用户
          </Button>
          <Button className="btn-secondary" onClick={() => setActiveDialog("create-admin")} disabled={tenant?.status === "disabled"}>
            创建租户管理员
          </Button>
          <Button icon={<ReloadOutlined />} onClick={() => void loadUsers()} disabled={loading}>
            刷新
          </Button>
        </div>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      {tenant && (
        <section className="panel tenant-admin-summary">
          <div className="tenant-admin-title">
            <div>
              <strong>{tenant.tenant_name}</strong>
              <span>{tenant.description || "暂无描述"}</span>
            </div>
            <TenantStatusBadge status={tenant.status} />
          </div>
          <dl className="tenant-admin-facts">
            <div>
              <dt>租户编码</dt>
              <dd>{tenant.tenant_code}</dd>
            </div>
            <div>
              <dt>用户数</dt>
              <dd>{tenant.user_count ?? members.length}</dd>
            </div>
            <div>
              <dt>Tenant Admin</dt>
              <dd>{tenant.tenant_admin_count ?? activeAdminCount}</dd>
            </div>
          </dl>
        </section>
      )}

      <div className="panel platform-table-wrap">
        <Table
          rowKey="user_id"
          columns={columns}
          dataSource={members}
          loading={loading}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          locale={{ emptyText: "该租户暂无成员。" }}
        />
      </div>

      {activeDialog === "add-existing" && (
        <div className="platform-modal-backdrop" role="presentation" onClick={closeDialog}>
          <section className="platform-modal" role="dialog" aria-modal="true" aria-labelledby="add-existing-user-title" onClick={(event) => event.stopPropagation()}>
            <div className="modal-title-row">
              <div>
                <span>成员接入</span>
                <strong id="add-existing-user-title">添加已有用户</strong>
              </div>
              <Button type="text" onClick={closeDialog}>关闭</Button>
            </div>
            <form className="platform-search-form" onSubmit={(event) => void onSearchExistingUser(event)}>
              <label className="field">
                <span>搜索用户</span>
                <input value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} placeholder="用户名、邮箱或手机号" autoFocus />
              </label>
              <Button type="primary" htmlType="submit" loading={searching}>
                {searching ? "搜索中..." : "搜索"}
              </Button>
            </form>
            <div className="platform-search-results">
              {searchResults.map((user) => {
                const alreadyMember = memberIds.has(user.id);
                return (
                  <div className="platform-search-result" key={user.id}>
                    <div>
                      <strong>{user.nickname || user.username || "未命名用户"}</strong>
                      <span>{[user.username, user.email, user.phone].filter(Boolean).join(" · ") || `用户 ID：${user.id}`}</span>
                    </div>
                    <Button
                      htmlType="button"
                      onClick={() => void onAddExistingUser(user)}
                      disabled={alreadyMember || Boolean(workingKey)}
                    >
                      {alreadyMember ? "已在租户" : workingKey === `add-user-${user.id}` ? "加入中..." : "加入租户"}
                    </Button>
                  </div>
                );
              })}
              {hasSearched && !searching && searchResults.length === 0 && <div className="empty-state">未找到匹配用户。</div>}
            </div>
          </section>
        </div>
      )}

      <Drawer
        title="创建租户管理员"
        width={520}
        open={activeDialog === "create-admin"}
        onClose={closeCreateAdminDrawer}
        destroyOnClose
        footer={
          <div className="drawer-footer-actions">
            <Button onClick={closeCreateAdminDrawer} disabled={workingKey === "create-admin"}>取消</Button>
            <Button type="primary" htmlType="submit" form="create-admin-form" loading={workingKey === "create-admin"}>创建管理员</Button>
          </div>
        }
      >
        <p className="drawer-subtitle">为当前租户创建一个 Tenant Admin 账号。</p>
        <form className="platform-drawer-form" id="create-admin-form" onSubmit={(event) => void onCreateTenantAdmin(event)}>
          <fieldset className="drawer-form-section">
            <legend>基础信息</legend>
            <label className="field">
              <span>用户名</span>
              <input value={adminForm.username} onChange={(event) => setAdminForm((form) => ({ ...form, username: event.target.value }))} placeholder="请输入用户名" autoFocus />
            </label>
            <label className="field">
              <span>姓名</span>
              <input value={adminForm.displayName} onChange={(event) => setAdminForm((form) => ({ ...form, displayName: event.target.value }))} placeholder="请输入姓名" />
            </label>
            <label className="field">
              <span>邮箱</span>
              <input value={adminForm.email} type="email" onChange={(event) => setAdminForm((form) => ({ ...form, email: event.target.value }))} placeholder="请输入邮箱" />
            </label>
            <label className="field">
              <span>手机号</span>
              <input value={adminForm.phone} inputMode="tel" onChange={(event) => setAdminForm((form) => ({ ...form, phone: event.target.value }))} placeholder="请输入手机号" />
            </label>
          </fieldset>
          <fieldset className="drawer-form-section">
            <legend>账号安全</legend>
            <Radio.Group className="password-mode-group" value={passwordMode} onChange={(event) => setPasswordMode(event.target.value)}>
              <Radio value="generated">使用默认初始密码</Radio>
              <Radio value="custom">自定义初始密码</Radio>
            </Radio.Group>
            {passwordMode === "custom" && (
              <label className="field">
                <span>自定义初始密码</span>
                <input value={adminForm.password} type="password" onChange={(event) => setAdminForm((form) => ({ ...form, password: event.target.value }))} placeholder="请输入初始密码" />
              </label>
            )}
          </fieldset>
        </form>
      </Drawer>
    </section>
  );
}

function MemberStatusBadge({ status }: { status: string }) {
  const active = status === "active";
  return <Tag className="soft-tag" color={active ? "success" : "error"}>{active ? "有效" : "已停用"}</Tag>;
}

function TenantAdminBadge({ enabled }: { enabled: boolean }) {
  return <Tag className="soft-tag" color={enabled ? "blue" : "default"}>{enabled ? "是" : "否"}</Tag>;
}
