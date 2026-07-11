import { Alert, Drawer, Form, Input, Space, Typography } from "antd";
import { useEffect, useState } from "react";
import { isBuiltinTenantRole } from "../../api/rbac";
import type { PermissionDTO, TenantRoleDTO } from "../../types";
import { PermissionSelector } from "./PermissionSelector";

export interface RoleEditorValues {
  code: string;
  name: string;
  description: string;
  permissionCodes: string[];
}

interface RoleEditorDrawerProps {
  open: boolean;
  role?: TenantRoleDTO | null;
  permissions: PermissionDTO[];
  initialPermissionCodes: string[];
  loading?: boolean;
  saving?: boolean;
  canManage: boolean;
  onClose: () => void;
  onSave: (values: RoleEditorValues) => void;
}

export function RoleEditorDrawer({ open, role, permissions, initialPermissionCodes, loading, saving, canManage, onClose, onSave }: RoleEditorDrawerProps) {
  const [form] = Form.useForm<RoleEditorValues>();
  const [permissionCodes, setPermissionCodes] = useState<string[]>([]);
  const editing = Boolean(role);
  const readonly = Boolean(role && isBuiltinTenantRole(role)) || !canManage || role?.status === "DISABLED";

  useEffect(() => {
    if (!open) return;
    const nextPermissions = initialPermissionCodes;
    setPermissionCodes(nextPermissions);
    form.setFieldsValue({
      code: role?.code ?? "",
      name: role?.name ?? "",
      description: role?.description ?? "",
      permissionCodes: nextPermissions
    });
  }, [form, initialPermissionCodes, open, role]);

  function submit(values: RoleEditorValues) {
    onSave({ ...values, permissionCodes });
  }

  return (
    <Drawer destroyOnClose open={open} title={editing ? "编辑角色" : "创建角色"} width={620} onClose={onClose}>
      {readonly && (
        <Alert
          className="tenant-rbac-drawer-alert"
          message={role && isBuiltinTenantRole(role) ? "系统内置角色由系统管理，当前仅可查看。" : "当前账号没有角色管理权限，当前仅可查看。"}
          type="info"
          showIcon
        />
      )}
      <Form form={form} layout="vertical" onFinish={submit}>
        <Form.Item label="角色 code" name="code" rules={[{ required: true, message: "请输入角色 code" }]}>
          <Input disabled={editing || readonly} placeholder="例如 SRE_ENGINEER" />
        </Form.Item>
        <Form.Item label="角色名称" name="name" rules={[{ required: true, message: "请输入角色名称" }]}>
          <Input disabled={readonly} placeholder="例如 SRE 工程师" />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea disabled={readonly} placeholder="描述该角色适用的业务职责" rows={3} />
        </Form.Item>
        <div className="tenant-rbac-drawer-section">
          <Space align="center">
            <Typography.Text strong>权限配置</Typography.Text>
            {loading && <Typography.Text type="secondary">正在加载权限...</Typography.Text>}
          </Space>
          <PermissionSelector disabled={readonly || loading} permissions={permissions} value={permissionCodes} onChange={setPermissionCodes} />
        </div>
        <div className="drawer-actions">
          <button className="secondary-action" type="button" onClick={onClose}>
            取消
          </button>
          <button className="primary-action" disabled={readonly || saving || loading} type="submit">
            保存
          </button>
        </div>
      </Form>
    </Drawer>
  );
}

