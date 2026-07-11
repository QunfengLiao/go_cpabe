import { Button, Input, Select, Space, Table, Tag, type TablePaginationConfig } from "antd";
import { EditOutlined, ReloadOutlined } from "@ant-design/icons";
import type { OrgMember, OrgMemberPage, OrgUnitNode } from "../../api/tenantOrg";
import { flattenUnits } from "./OrgUnitTreePanel";

interface OrgMemberTableProps {
  data: OrgMemberPage;
  units: OrgUnitNode[];
  loading?: boolean;
  filters: { keyword: string; orgUnitId: number | undefined; status: "active" | "inactive" | "all" };
  onFiltersChange: (filters: OrgMemberTableProps["filters"]) => void;
  onPageChange: (page: number, pageSize: number) => void;
  onRefresh: () => void;
  onEdit: (member: OrgMember) => void;
  canManage?: boolean;
}

export function OrgMemberTable({ data, units, loading, filters, onFiltersChange, onPageChange, onRefresh, onEdit, canManage = true }: OrgMemberTableProps) {
  const unitOptions = flattenUnits(units).map((unit) => ({ label: unit.name, value: unit.id }));
  const pagination: TablePaginationConfig = {
    current: data.page,
    pageSize: data.pageSize,
    total: data.total,
    showSizeChanger: true,
    onChange: onPageChange
  };

  return (
    <section className="tenant-org-member-table">
      <div className="tenant-org-table-toolbar">
        <Space wrap>
          <Input.Search
            allowClear
            placeholder="搜索用户名、昵称或邮箱"
            value={filters.keyword}
            onChange={(event) => onFiltersChange({ ...filters, keyword: event.target.value })}
            onSearch={(keyword) => onFiltersChange({ ...filters, keyword })}
            style={{ width: 240 }}
          />
          <Select
            allowClear
            placeholder="部门"
            options={unitOptions}
            value={filters.orgUnitId}
            onChange={(orgUnitId) => onFiltersChange({ ...filters, orgUnitId })}
            style={{ width: 180 }}
          />
          <Select
            value={filters.status}
            options={[
              { label: "有效", value: "active" },
              { label: "已移除", value: "inactive" },
              { label: "全部", value: "all" }
            ]}
            onChange={(status) => onFiltersChange({ ...filters, status })}
            style={{ width: 120 }}
          />
        </Space>
        <Button icon={<ReloadOutlined />} onClick={onRefresh}>
          刷新
        </Button>
      </div>
      <Table
        rowKey="id"
        dataSource={data.items}
        loading={loading}
        pagination={pagination}
        columns={[
          {
            title: "成员",
            dataIndex: "nickname",
            render: (_, member) => (
              <div className="member-cell">
                <strong>{member.nickname || member.username || member.email}</strong>
                <span>{member.email}</span>
              </div>
            )
          },
          {
            title: "部门",
            dataIndex: ["orgUnit", "name"],
            render: (_, member) => (
              <Space>
                <span>{member.orgUnit.name}</span>
                {member.isPrimary && <Tag color="blue">主部门</Tag>}
              </Space>
            )
          },
          {
            title: "部门职务",
            dataIndex: "positions",
            render: (positions: string[]) => positions.length ? positions.map((position) => <Tag key={position}>{positionLabel(position)}</Tag>) : <Tag>普通成员</Tag>
          },
          {
            title: "系统角色",
            dataIndex: "systemRoles",
            render: (roles: string[]) => roles.map((role) => <Tag key={role}>{role}</Tag>)
          },
          {
            title: "状态",
            dataIndex: "memberStatus",
            render: (status) => <Tag color={status === "active" ? "green" : "default"}>{status === "active" ? "有效" : "已移除"}</Tag>
          },
          {
            title: "操作",
            width: 120,
            render: (_, member) => (
              <Button disabled={!canManage} icon={<EditOutlined />} onClick={() => onEdit(member)} type="link">
                编辑
              </Button>
            )
          }
        ]}
      />
    </section>
  );
}

function positionLabel(position: string) {
  if (position === "ORG_LEADER") return "负责人";
  if (position === "DEPUTY_LEADER") return "副负责人";
  return position;
}
