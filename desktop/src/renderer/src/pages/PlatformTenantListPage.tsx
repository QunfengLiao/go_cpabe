import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Button, Drawer, Table, Tag, type TableColumnsType } from "antd";
import { PlusSquareOutlined, ReloadOutlined } from "@ant-design/icons";
import { createPlatformTenant, listPlatformTenants } from "../api/platform";
import { ApiError } from "../api/request";
import { Alert } from "../components/Alert";
import type { TenantSummary } from "../types";

const TENANT_CODE_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export function PlatformTenantListPage() {
  const [tenants, setTenants] = useState<TenantSummary[]>([]);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "enabled" | "disabled">("all");
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [createForm, setCreateForm] = useState({ name: "", code: "", description: "" });
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  async function loadTenants() {
    setLoading(true);
    setError("");
    try {
      setTenants(await listPlatformTenants());
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "获取租户列表失败");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadTenants();
  }, []);

  const filteredTenants = useMemo(() => {
    const value = keyword.trim().toLowerCase();
    return tenants.filter((tenant) => {
      const matchesKeyword = !value || [tenant.tenant_name, tenant.tenant_code, tenant.description].some((item) => item?.toLowerCase().includes(value));
      const matchesStatus = statusFilter === "all" || tenant.status === statusFilter;
      return matchesKeyword && matchesStatus;
    });
  }, [keyword, statusFilter, tenants]);

  const columns = useMemo<TableColumnsType<TenantSummary>>(() => [
    {
      title: "租户名称",
      dataIndex: "tenant_name",
      render: (_, tenant) => (
        <span className="table-title-cell">
          <strong>{tenant.tenant_name}</strong>
          <small>{tenant.description || "暂无描述"}</small>
        </span>
      )
    },
    { title: "编码", dataIndex: "tenant_code", render: (value: string) => <code className="table-code">{value}</code> },
    { title: "状态", dataIndex: "status", width: 96, render: (value?: string) => <TenantStatusBadge status={value} /> },
    { title: "用户数", dataIndex: "user_count", width: 96, render: (value?: number) => value ?? 0 },
    { title: "Tenant Admin", dataIndex: "tenant_admin_count", width: 130, render: (value?: number) => value ?? 0 },
    { title: "创建时间", dataIndex: "created_at", width: 160, render: (value?: string) => formatDate(value) },
    {
      title: "操作",
      key: "actions",
      width: 150,
      render: (_, tenant) => (
        <div className="inline-action-row">
          <Link to={`/platform/tenants/${tenant.tenant_id}`}>详情</Link>
          <Link to={`/platform/tenants/${tenant.tenant_id}/users`}>成员</Link>
        </div>
      )
    }
  ], []);

  async function onCreateTenant(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");
    const name = createForm.name.trim();
    const code = createForm.code.trim().toLowerCase();
    if (!name) {
      setError("请输入租户名称");
      return;
    }
    if (!TENANT_CODE_PATTERN.test(code)) {
      setError("租户编码只能包含小写字母、数字和中划线");
      return;
    }
    setSaving(true);
    try {
      await createPlatformTenant({ name, code, description: createForm.description.trim() });
      setCreateForm({ name: "", code: "", description: "" });
      setDrawerOpen(false);
      setSuccess("租户已创建");
      await loadTenants();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "创建租户失败");
    } finally {
      setSaving(false);
    }
  }

  function closeCreateDrawer() {
    if (saving) return;
    setDrawerOpen(false);
    setCreateForm({ name: "", code: "", description: "" });
  }

  return (
    <section className="platform-page">
      <div className="platform-header">
        <div>
          <h2>租户列表</h2>
          <p>管理平台下的组织边界、启用状态和租户管理员数量</p>
        </div>
        <div className="platform-actions">
          <Button icon={<ReloadOutlined />} onClick={() => void loadTenants()} disabled={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<PlusSquareOutlined />} onClick={() => setDrawerOpen(true)}>
            创建租户
          </Button>
        </div>
      </div>

      <Alert type="error" message={error} />
      <Alert type="success" message={success} autoDismissMs={3000} onDismiss={() => setSuccess("")} />

      <div className="panel platform-filter platform-filter-row">
        <label className="field">
          <span>搜索租户</span>
          <input value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="按名称、编码或描述搜索" />
        </label>
        <label className="field">
          <span>状态筛选</span>
          <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as "all" | "enabled" | "disabled")}>
            <option value="all">全部状态</option>
            <option value="enabled">仅启用</option>
            <option value="disabled">仅禁用</option>
          </select>
        </label>
      </div>

      <div className="panel platform-table-wrap">
        <Table
          rowKey="tenant_id"
          columns={columns}
          dataSource={filteredTenants}
          loading={loading}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          locale={{ emptyText: "没有匹配的租户。" }}
        />
      </div>

      <Drawer
        title="创建租户"
        width={520}
        open={drawerOpen}
        onClose={closeCreateDrawer}
        destroyOnClose
        extra={<Button onClick={closeCreateDrawer} disabled={saving}>取消</Button>}
        footer={
          <div className="drawer-footer-actions">
            <Button onClick={closeCreateDrawer} disabled={saving}>取消</Button>
            <Button type="primary" form="create-tenant-form" htmlType="submit" loading={saving}>创建租户</Button>
          </div>
        }
      >
        <p className="drawer-subtitle">为新的组织边界创建租户。</p>
        <form className="platform-drawer-form" id="create-tenant-form" onSubmit={(event) => void onCreateTenant(event)}>
          <label className="field">
            <span>租户名称</span>
            <input value={createForm.name} maxLength={128} onChange={(event) => setCreateForm((form) => ({ ...form, name: event.target.value }))} placeholder="请输入租户名称" autoFocus />
          </label>
          <label className="field">
            <span>租户编码</span>
            <input value={createForm.code} maxLength={64} onChange={(event) => setCreateForm((form) => ({ ...form, code: event.target.value.toLowerCase() }))} placeholder="请输入租户编码" />
          </label>
          <label className="field">
            <span>租户描述</span>
            <textarea value={createForm.description} maxLength={512} onChange={(event) => setCreateForm((form) => ({ ...form, description: event.target.value }))} placeholder="请输入租户描述" />
          </label>
        </form>
      </Drawer>
    </section>
  );
}

export function TenantStatusBadge({ status }: { status?: string }) {
  const enabled = status === "enabled";
  return <Tag color={enabled ? "success" : "error"}>{enabled ? "启用" : "禁用"}</Tag>;
}

export function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}
