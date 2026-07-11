import { Alert, Button, Modal, Result, Skeleton, Typography, message } from "antd";
import { PlusOutlined, ReloadOutlined } from "@ant-design/icons";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  createTenantRole,
  disableTenantRole,
  filterTenantVisibleRoles,
  getTenantRolePermissions,
  isBuiltinTenantRole,
  listTenantPermissions,
  listTenantRoles,
  rbacErrorMessage,
  replaceTenantRolePermissions,
  updateTenantRole
} from "../api/rbac";
import { useAuth } from "../auth/AuthContext";
import { RoleDetailDrawer } from "../components/tenant-rbac/RoleDetailPanel";
import { RoleEditorDrawer, type RoleEditorValues } from "../components/tenant-rbac/RoleEditorDrawer";
import { RoleList } from "../components/tenant-rbac/RoleList";
import type { PermissionDTO, TenantRoleDTO } from "../types";

export function TenantRolesPage() {
  const auth = useAuth();
  const canRead = auth.hasPermission("tenant.role.read");
  const canManage = auth.hasPermission("tenant.role.manage");
  const [roles, setRoles] = useState<TenantRoleDTO[]>([]);
  const [permissions, setPermissions] = useState<PermissionDTO[]>([]);
  const [selectedRoleId, setSelectedRoleId] = useState<number>();
  const [selectedPermissions, setSelectedPermissions] = useState<PermissionDTO[]>([]);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailError, setDetailError] = useState("");
  const [listError, setListError] = useState("");
  const [loading, setLoading] = useState(false);
  const [loadingPermissions, setLoadingPermissions] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingRole, setEditingRole] = useState<TenantRoleDTO | null>(null);
  const requestSeq = useRef(0);
  const refreshingRef = useRef(false);
  const permissionSeq = useRef(0);
  const selectedRole = useMemo(() => roles.find((role) => role.id === selectedRoleId) ?? null, [roles, selectedRoleId]);
  const selectedPermissionCodes = selectedPermissions.map((permission) => permission.code);
  const tenantId = Number(auth.currentTenantId) || undefined;

  useEffect(() => {
    requestSeq.current += 1;
    permissionSeq.current += 1;
    refreshingRef.current = false;
    setRoles([]);
    setPermissions([]);
    setSelectedRoleId(undefined);
    setSelectedPermissions([]);
    setDetailOpen(false);
    setDetailError("");
    setListError("");
    setLoading(false);
    setRefreshing(false);
    setLoadingPermissions(false);
    if (canRead && tenantId) void refreshAll();
  }, [auth.currentTenantId, auth.currentUserId, canRead]);

  async function refreshAll(nextSelectedRoleId = selectedRoleId) {
    if (refreshingRef.current) return;
    const seq = ++requestSeq.current;
    refreshingRef.current = true;
    setRefreshing(true);
    setLoading(true);
    setListError("");
    try {
      const [roleData, permissionData] = await Promise.all([listTenantRoles(), listTenantPermissions()]);
      if (seq !== requestSeq.current) return;
      const tenantRoles = filterTenantVisibleRoles(roleData, tenantId);
      setRoles(tenantRoles);
      setPermissions(permissionData.filter((permission) => permission.scopeType === "TENANT"));
      const nextRole = nextSelectedRoleId ? tenantRoles.find((role) => role.id === nextSelectedRoleId) : null;
      if (detailOpen && nextSelectedRoleId && !nextRole) {
        closeDetail();
        message.warning("当前角色已不存在或不属于当前租户，详情已关闭");
        return;
      }
      if (nextRole && detailOpen) {
        setSelectedRoleId(nextRole.id);
        await loadRolePermissions(nextRole.id);
      }
    } catch (err) {
      if (seq !== requestSeq.current) return;
      setListError(rbacErrorMessage(err, "角色列表加载失败"));
    } finally {
      setLoading(false);
      setRefreshing(false);
      refreshingRef.current = false;
    }
  }

  async function loadRolePermissions(roleId: number) {
    const seq = ++permissionSeq.current;
    setLoadingPermissions(true);
    setDetailError("");
    try {
      const data = await getTenantRolePermissions(roleId);
      if (seq !== permissionSeq.current) return;
      setSelectedPermissions((data.permissions ?? []).filter((permission) => permission.scopeType === "TENANT"));
    } catch (err) {
      if (seq !== permissionSeq.current) return;
      setSelectedPermissions([]);
      setDetailError(rbacErrorMessage(err, "角色详情加载失败，角色可能已被禁用、删除或不属于当前租户"));
    } finally {
      if (seq === permissionSeq.current) setLoadingPermissions(false);
    }
  }

  function openDetail(role: TenantRoleDTO) {
    setSelectedRoleId(role.id);
    setDetailOpen(true);
    void loadRolePermissions(role.id);
  }

  function closeDetail() {
    setDetailOpen(false);
    setSelectedRoleId(undefined);
    setSelectedPermissions([]);
    setDetailError("");
  }

  function openCreate() {
    setEditingRole(null);
    setSelectedPermissions([]);
    setEditorOpen(true);
  }

  async function openEdit(role: TenantRoleDTO) {
    setEditingRole(role);
    setSelectedRoleId(role.id);
    await loadRolePermissions(role.id);
    setEditorOpen(true);
  }

  async function saveRole(values: RoleEditorValues) {
    setSaving(true);
    try {
      let nextSelectedRoleId = editingRole?.id;
      if (editingRole) {
        await updateTenantRole(editingRole.id, { name: values.name, description: values.description });
        if (!isBuiltinTenantRole(editingRole)) {
          await replaceTenantRolePermissions(editingRole.id, values.permissionCodes);
          await auth.refreshAuthorization();
        }
        message.success("角色已更新");
      } else {
        const created = await createTenantRole({
          code: values.code,
          name: values.name,
          description: values.description,
          permissionCodes: values.permissionCodes
        });
        nextSelectedRoleId = created.id;
        setSelectedRoleId(created.id);
        setDetailOpen(true);
        message.success("角色已创建");
      }
      setEditorOpen(false);
      await refreshAll(nextSelectedRoleId);
      if (nextSelectedRoleId) {
        setDetailOpen(true);
        await loadRolePermissions(nextSelectedRoleId);
      }
    } catch (err) {
      message.error(rbacErrorMessage(err, "角色保存失败"));
    } finally {
      setSaving(false);
    }
  }

  function confirmDisable(role: TenantRoleDTO) {
    const affectsCurrentUser = auth.tenantRoles.includes(role.code);
    Modal.confirm({
      title: "禁用角色",
      content: (
        <div className="tenant-rbac-confirm">
          <p>确认禁用“{role.name}”？</p>
          <p>禁用后，该角色不能继续产生权限；现有成员关系会保留为历史绑定，但不会再为成员授权。</p>
          {affectsCurrentUser && <p>当前登录用户拥有该角色，禁用后当前账号的授权上下文可能立即变化。</p>}
        </div>
      ),
      okText: "禁用角色",
      okButtonProps: { danger: true },
      cancelText: "取消",
      async onOk() {
        try {
          await disableTenantRole(role.id);
          message.success("角色已禁用");
          await auth.refreshAuthorization();
          await refreshAll(role.id);
        } catch (err) {
          message.error(rbacErrorMessage(err, "角色禁用失败"));
        }
      }
    });
  }

  if (auth.authorizationStatus === "ready" && !canRead) {
    return (
      <Result
        status="403"
        title="无权访问角色管理"
        subTitle="当前账号缺少 tenant.role.read 权限。"
      />
    );
  }

  return (
    <div className="tenant-rbac-page">
      <div className="page-heading">
        <div>
          <Typography.Title level={2}>角色管理</Typography.Title>
          <Typography.Text type="secondary">维护当前租户的内置角色和自定义业务角色</Typography.Text>
        </div>
        <div className="tenant-rbac-page-actions">
          <Button disabled={refreshing} icon={<ReloadOutlined />} loading={refreshing} onClick={() => void refreshAll()}>
            刷新
          </Button>
          <Button disabled={!canManage} icon={<PlusOutlined />} type="primary" onClick={openCreate}>
            创建角色
          </Button>
        </div>
      </div>

      {listError && <Alert message={listError} showIcon type="error" />}
      {loading && roles.length === 0 ? (
        <section className="tenant-rbac-list-panel">
          <Skeleton active paragraph={{ rows: 8 }} />
        </section>
      ) : (
        <RoleList
          canManage={canManage}
          loading={loading}
          roles={roles}
          selectedRoleId={detailOpen ? selectedRoleId : undefined}
          onDisable={confirmDisable}
          onEdit={(role) => { void openEdit(role); }}
          onView={openDetail}
        />
      )}

      <RoleDetailDrawer
        canManage={canManage}
        error={detailError}
        loading={loadingPermissions}
        open={detailOpen}
        permissions={selectedPermissions}
        role={selectedRole}
        onClose={closeDetail}
        onEdit={(role) => { void openEdit(role); }}
      />

      <RoleEditorDrawer
        canManage={canManage}
        initialPermissionCodes={editingRole ? selectedPermissionCodes : []}
        loading={loadingPermissions}
        open={editorOpen}
        permissions={permissions}
        role={editingRole}
        saving={saving}
        onClose={() => setEditorOpen(false)}
        onSave={(values) => { void saveRole(values); }}
      />
    </div>
  );
}
