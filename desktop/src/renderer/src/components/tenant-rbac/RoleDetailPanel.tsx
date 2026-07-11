import { Alert, Button, Collapse, Descriptions, Drawer, Empty, Skeleton, Space, Tag, Tooltip, Typography } from "antd";
import { EditOutlined } from "@ant-design/icons";
import { isBuiltinTenantRole } from "../../api/rbac";
import type { PermissionDTO, TenantRoleDTO } from "../../types";
import { categoryLabel, normalizeCategory } from "./RoleList";

interface RoleDetailDrawerProps {
  open: boolean;
  role?: TenantRoleDTO | null;
  permissions: PermissionDTO[];
  loading?: boolean;
  error?: string;
  canManage: boolean;
  onClose: () => void;
  onEdit: (role: TenantRoleDTO) => void;
}

export function RoleDetailDrawer({ open, role, permissions, loading, error, canManage, onClose, onEdit }: RoleDetailDrawerProps) {
  const builtin = role ? isBuiltinTenantRole(role) : false;
  const editable = Boolean(role && !builtin && canManage && role.status === "ACTIVE");
  const tenantPermissions = permissions.filter((permission) => permission.scopeType === "TENANT");
  const permissionGroups = groupPermissionsByResource(tenantPermissions);

  return (
    <Drawer
      className="tenant-rbac-detail-drawer"
      destroyOnClose
      footer={
        <div className="tenant-rbac-drawer-footer">
          <Button onClick={onClose}>关闭</Button>
          {role && editable && (
            <Button icon={<EditOutlined />} type="primary" onClick={() => onEdit(role)}>
              编辑角色
            </Button>
          )}
        </div>
      }
      open={open}
      title={role ? <RoleDrawerTitle role={role} /> : "角色详情"}
      width={500}
      onClose={onClose}
    >
      {loading && !role ? (
        <Skeleton active paragraph={{ rows: 8 }} />
      ) : role ? (
        <div className="tenant-rbac-detail-content">
          {error && <Alert message={error} showIcon type="error" />}
          {builtin && (
            <Alert
              message="该角色由系统维护，角色编码、分类、作用域和默认权限不可由租户修改。"
              showIcon
              type="info"
            />
          )}
          {role.status === "DISABLED" && <Alert message="该角色已被禁用，不再产生有效权限。" showIcon type="warning" />}

          <section className="tenant-rbac-detail-section">
            <Typography.Text strong>基本信息</Typography.Text>
            <Descriptions column={1} size="small" className="tenant-rbac-descriptions">
              <Descriptions.Item label="角色名称">{role.name}</Descriptions.Item>
              <Descriptions.Item label="code">{role.code}</Descriptions.Item>
              <Descriptions.Item label="分类">{categoryLabel(normalizeCategory(role))}</Descriptions.Item>
              <Descriptions.Item label="来源">{builtin ? "系统内置" : "租户自定义"}</Descriptions.Item>
              <Descriptions.Item label="状态">{role.status === "ACTIVE" ? "启用" : "禁用"}</Descriptions.Item>
              <Descriptions.Item label="成员数量">{role.activeMemberCount ?? 0}</Descriptions.Item>
              <Descriptions.Item label="权限数量">{role.permissionCount ?? tenantPermissions.length}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDate(role.createdAt ?? role.created_at)}</Descriptions.Item>
              <Descriptions.Item label="更新时间">{formatDate(role.updatedAt ?? role.updated_at)}</Descriptions.Item>
            </Descriptions>
          </section>

          <section className="tenant-rbac-detail-section">
            <Typography.Text strong>角色说明</Typography.Text>
            <Typography.Paragraph className="tenant-rbac-description">{role.description || "暂无描述"}</Typography.Paragraph>
          </section>

          <section className="tenant-rbac-detail-section">
            <Typography.Text strong>权限范围</Typography.Text>
            {loading ? (
              <Skeleton active paragraph={{ rows: 4 }} title={false} />
            ) : permissionGroups.length > 0 ? (
              <Collapse
                bordered={false}
                className="tenant-rbac-permission-collapse"
                defaultActiveKey={permissionGroups.map((group) => group.resourceType)}
                items={permissionGroups.map((group) => ({
                  key: group.resourceType,
                  label: (
                    <span className="tenant-rbac-permission-group-title">
                      <span>{resourceTypeLabel(group.resourceType)}</span>
                      <Tag>{group.items.length}</Tag>
                    </span>
                  ),
                  children: (
                    <div className="tenant-rbac-permission-tags">
                      {group.items.map((permission) => (
                        <Tooltip key={permission.code} title={permission.code}>
                          <Tag>
                            <span>{permission.name || permission.code}</span>
                            <small>{permission.code}</small>
                          </Tag>
                        </Tooltip>
                      ))}
                    </div>
                  )
                }))}
              />
            ) : (
              <Empty description="该角色暂未配置权限" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </section>
        </div>
      ) : (
        <Empty description="请选择一个角色查看详情" />
      )}
    </Drawer>
  );
}

function RoleDrawerTitle({ role }: { role: TenantRoleDTO }) {
  const builtin = isBuiltinTenantRole(role);
  return (
    <div className="tenant-rbac-drawer-title">
      <div>
        <Typography.Title level={4}>{role.name}</Typography.Title>
        <Typography.Text type="secondary">{role.code}</Typography.Text>
      </div>
      <Space size={4} wrap>
        <Tag>{categoryLabel(normalizeCategory(role))}</Tag>
        <Tag color={builtin ? "blue" : "green"}>{builtin ? "系统内置" : "租户自定义"}</Tag>
        <Tag color={role.status === "ACTIVE" ? "green" : "default"}>{role.status === "ACTIVE" ? "启用" : "禁用"}</Tag>
      </Space>
    </div>
  );
}

function groupPermissionsByResource(permissions: PermissionDTO[]): Array<{ resourceType: string; items: PermissionDTO[] }> {
  const groups = new Map<string, PermissionDTO[]>();
  permissions.forEach((permission) => {
    const resourceType = permission.resourceType || "other";
    groups.set(resourceType, [...(groups.get(resourceType) ?? []), permission]);
  });
  return Array.from(groups.entries()).map(([resourceType, items]) => ({ resourceType, items }));
}

function resourceTypeLabel(resourceType: string): string {
  const labels: Record<string, string> = {
    dashboard: "仪表盘",
    tenant: "租户管理",
    role: "角色管理",
    member: "成员管理",
    org: "组织管理",
    policy: "访问策略",
    file: "文件管理",
    audit: "审计"
  };
  return labels[resourceType] ?? resourceType;
}

function formatDate(value?: string) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return date.toLocaleString();
}
