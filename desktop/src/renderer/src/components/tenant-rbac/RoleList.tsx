import { Button, Dropdown, Empty, Input, Select, Space, Table, Tag, Tooltip, Typography, type MenuProps } from "antd";
import { EditOutlined, EyeOutlined, MoreOutlined, StopOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { isBuiltinTenantRole } from "../../api/rbac";
import type { TenantRoleCategory, TenantRoleDTO, TenantRoleStatus } from "../../types";

interface RoleListProps {
  roles: TenantRoleDTO[];
  loading?: boolean;
  selectedRoleId?: number;
  canManage: boolean;
  onView: (role: TenantRoleDTO) => void;
  onEdit: (role: TenantRoleDTO) => void;
  onDisable: (role: TenantRoleDTO) => void;
}

const categoryOptions = [
  { label: "全部分类", value: "ALL" },
  { label: "治理角色", value: "GOVERNANCE" },
  { label: "业务角色", value: "BUSINESS" },
  { label: "能力角色", value: "CAPABILITY" }
];

const statusOptions = [
  { label: "全部状态", value: "ALL" },
  { label: "启用", value: "ACTIVE" },
  { label: "禁用", value: "DISABLED" }
];

export function RoleList({ roles, loading, selectedRoleId, canManage, onView, onEdit, onDisable }: RoleListProps) {
  const [keyword, setKeyword] = useState("");
  const [category, setCategory] = useState<TenantRoleCategory | "ALL">("ALL");
  const [status, setStatus] = useState<TenantRoleStatus | "ALL">("ALL");

  const filtered = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    return roles
      .filter((role) => role.scopeType === "TENANT" && role.code !== "PLATFORM_ADMIN")
      .filter((role) => {
        const roleCategory = normalizeCategory(role);
        if (category !== "ALL" && roleCategory !== category) return false;
        if (status !== "ALL" && role.status !== status) return false;
        if (!query) return true;
        return `${role.name} ${role.code} ${role.description ?? ""}`.toLowerCase().includes(query);
      });
  }, [category, keyword, roles, status]);

  const emptyText = roles.length === 0 ? "当前租户暂无可展示角色" : "没有匹配的角色";

  return (
    <section className="tenant-rbac-list-panel">
      <div className="tenant-rbac-toolbar">
        <div className="tenant-rbac-filters">
          <Input.Search allowClear placeholder="搜索角色名称、Code 或描述" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
          <Select options={categoryOptions} value={category} onChange={setCategory} />
          <Select options={statusOptions} value={status} onChange={setStatus} />
        </div>
        <Typography.Text className="tenant-rbac-count" type="secondary">共 {filtered.length} 个角色</Typography.Text>
      </div>

      <Table
        rowKey="id"
        dataSource={filtered}
        loading={loading}
        locale={{ emptyText: <Empty description={emptyText} image={Empty.PRESENTED_IMAGE_SIMPLE} /> }}
        pagination={{ pageSize: 10, size: "small", showSizeChanger: false }}
        rowClassName={(role) => role.id === selectedRoleId ? "tenant-rbac-row-selected" : ""}
        scroll={{ x: 980 }}
        size="small"
        tableLayout="fixed"
        onRow={(role) => ({ onClick: () => onView(role) })}
        columns={[
          {
            title: "角色",
            dataIndex: "name",
            width: 220,
            render: (_, role) => (
              <div className="tenant-rbac-role-cell">
                <Tooltip title={role.name}>
                  <strong>{role.name}</strong>
                </Tooltip>
                <span>{role.code}</span>
              </div>
            )
          },
          {
            title: "分类",
            width: 104,
            render: (_, role) => <Tag>{categoryLabel(normalizeCategory(role))}</Tag>
          },
          {
            title: "来源",
            width: 108,
            render: (_, role) => <Tag color={isBuiltinTenantRole(role) ? "blue" : "green"}>{isBuiltinTenantRole(role) ? "系统内置" : "租户自定义"}</Tag>
          },
          {
            title: "状态",
            width: 84,
            render: (_, role) => <Tag color={role.status === "ACTIVE" ? "green" : "default"}>{role.status === "ACTIVE" ? "启用" : "禁用"}</Tag>
          },
          { title: "权限数", dataIndex: "permissionCount", align: "right", width: 88, render: (value) => value ?? 0 },
          { title: "成员数", dataIndex: "activeMemberCount", align: "right", width: 88, render: (value) => value ?? 0 },
          {
            title: "操作",
            width: 148,
            fixed: "right",
            render: (_, role) => (
              <div className="tenant-rbac-actions" onClick={(event) => event.stopPropagation()}>
                <Button icon={<EyeOutlined />} size="small" type="link" onClick={() => onView(role)}>
                  查看
                </Button>
                {renderRoleActions(role, canManage, onEdit, onDisable)}
              </div>
            )
          }
        ]}
      />
    </section>
  );
}

function renderRoleActions(role: TenantRoleDTO, canManage: boolean, onEdit: (role: TenantRoleDTO) => void, onDisable: (role: TenantRoleDTO) => void) {
  if (isBuiltinTenantRole(role)) return null;
  if (!canManage) return <Typography.Text type="secondary">只读</Typography.Text>;
  if (role.status === "DISABLED") return <Tag>已禁用</Tag>;

  const items: MenuProps["items"] = [
    { key: "permissions", label: "配置权限", icon: <EditOutlined /> },
    { type: "divider" },
    { key: "disable", label: "禁用角色", danger: true, icon: <StopOutlined /> }
  ];

  return (
    <Space size={2}>
      <Button icon={<EditOutlined />} size="small" type="text" onClick={() => onEdit(role)}>
        编辑
      </Button>
      <Dropdown
        menu={{
          items,
          onClick: ({ key }) => {
            if (key === "permissions") onEdit(role);
            if (key === "disable") onDisable(role);
          }
        }}
        trigger={["click"]}
      >
        <Button aria-label="更多角色操作" icon={<MoreOutlined />} size="small" type="text" />
      </Dropdown>
    </Space>
  );
}

export function normalizeCategory(role: TenantRoleDTO): TenantRoleCategory {
  return role.roleCategory ?? role.category ?? "BUSINESS";
}

export function categoryLabel(category: TenantRoleCategory): string {
  if (category === "GOVERNANCE") return "治理角色";
  if (category === "CAPABILITY") return "能力角色";
  return "业务角色";
}
