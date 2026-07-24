import { useEffect, useMemo, useState } from "react";
import { Button, Collapse, Descriptions, Dropdown, Empty, Modal, Tabs, Tag, Tooltip, Typography, message } from "antd";
import { ApartmentOutlined, DeleteOutlined, EditOutlined, MoreOutlined, PlusOutlined, SwapOutlined } from "@ant-design/icons";
import {
  addOrgMember,
  createOrgUnit,
  deleteOrgUnit,
  listOrgMembers,
  listOrgTree,
  moveOrgUnit,
  removeOrgMember,
  setOrgMemberPositions,
  setOrgMemberPrimary,
  updateOrgUnit,
  type OrgMember,
  type OrgMemberPage,
  type OrgUnitNode
} from "../api/tenantOrg";
import { useAuth } from "../auth/AuthContext";
import { OrgMemberDrawer, type OrgMemberDrawerValues } from "../components/tenant-org/OrgMemberDrawer";
import { OrgMemberTable } from "../components/tenant-org/OrgMemberTable";
import { OrgUnitDrawer, type OrgUnitDrawerMode, type OrgUnitDrawerValues } from "../components/tenant-org/OrgUnitDrawer";
import { OrgUnitTreePanel, flattenUnits } from "../components/tenant-org/OrgUnitTreePanel";
import { TenantImportDrawer } from "../components/tenant-import/TenantImportDrawer";

const emptyMemberPage: OrgMemberPage = { items: [], total: 0, page: 1, pageSize: 20 };

export function TenantOrgManagementPage() {
  const auth = useAuth();
  const tenantId = Number(auth.currentTenantId);
  const canManageOrg = auth.hasPermission("tenant.org.manage");
  const canImport = auth.hasPermission("tenant.import.manage");
  const [units, setUnits] = useState<OrgUnitNode[]>([]);
  const [selectedUnitId, setSelectedUnitId] = useState<number>();
  const [activeTab, setActiveTab] = useState("units");
  const [membersLoaded, setMembersLoaded] = useState(false);
  const [members, setMembers] = useState<OrgMemberPage>(emptyMemberPage);
  const [loadingTree, setLoadingTree] = useState(false);
  const [loadingMembers, setLoadingMembers] = useState(false);
  const [saving, setSaving] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [unitDrawer, setUnitDrawer] = useState<{ open: boolean; mode: OrgUnitDrawerMode; unit?: OrgUnitNode; parent?: OrgUnitNode }>({ open: false, mode: "create" });
  const [memberDrawer, setMemberDrawer] = useState<{ open: boolean; member?: OrgMember }>({ open: false });
  const [memberFilters, setMemberFilters] = useState({ keyword: "", orgUnitId: undefined as number | undefined, status: "active" as "active" | "inactive" | "all" });
  const selectedUnit = useMemo(() => flattenUnits(units).find((unit) => unit.id === selectedUnitId), [selectedUnitId, units]);
  const flatUnits = useMemo(() => flattenUnits(units), [units]);
  const selectedParent = useMemo(() => flatUnits.find((unit) => unit.id === selectedUnit?.parentId), [flatUnits, selectedUnit?.parentId]);
  const selectedLeader = selectedUnit?.leader;
  const selectedDeputyCount = selectedUnit?.deputyLeaderCount ?? 0;
  const selectedActiveMemberCount = selectedUnit?.memberCount ?? 0;
  const selectedPositionSummary = positionSummary(Boolean(selectedLeader), selectedDeputyCount);

  useEffect(() => {
    if (!tenantId) return;
    void loadTree();
  }, [tenantId]);

  useEffect(() => {
    if (activeTab !== "members") return;
    void loadMembers(1, members.pageSize);
  }, [activeTab, memberFilters.keyword, memberFilters.orgUnitId, memberFilters.status]);

  async function loadTree() {
    if (!tenantId) return;
    setLoadingTree(true);
    try {
      const data = await listOrgTree(true);
      setUnits(data);
      setSelectedUnitId((current) => current ?? data[0]?.id);
    } catch (err) {
      message.error(err instanceof Error ? err.message : "部门树加载失败");
    } finally {
      setLoadingTree(false);
    }
  }

  async function loadMembers(page = members.page, pageSize = members.pageSize) {
    setLoadingMembers(true);
    try {
      const data = await listOrgMembers({ ...memberFilters, page, pageSize });
      setMembers(data);
      setMembersLoaded(true);
    } catch (err) {
      message.error(err instanceof Error ? err.message : "成员列表加载失败");
    } finally {
      setLoadingMembers(false);
    }
  }

  async function saveUnit(values: OrgUnitDrawerValues) {
    setSaving(true);
    try {
      if (unitDrawer.mode === "create") {
        await createOrgUnit({ parentId: values.parentId, name: values.name ?? "", sortOrder: values.sortOrder ?? 0 });
      } else if (unitDrawer.mode === "edit" && unitDrawer.unit) {
        await updateOrgUnit(unitDrawer.unit.id, { name: values.name, sortOrder: values.sortOrder, status: values.status });
      } else if (unitDrawer.mode === "move" && unitDrawer.unit) {
        await moveOrgUnit(unitDrawer.unit.id, { targetParentId: values.parentId, sortOrder: values.sortOrder });
      }
      setUnitDrawer({ open: false, mode: "create" });
      await loadTree();
      if (membersLoaded) await loadMembers();
      message.success("部门已保存");
    } catch (err) {
      message.error(err instanceof Error ? err.message : "部门保存失败");
    } finally {
      setSaving(false);
    }
  }

  function confirmDelete(unit: OrgUnitNode) {
    Modal.confirm({
      title: "删除部门",
      content: `确认删除“${unit.name}”？有子部门或有效成员时后端会拒绝删除。`,
      okText: "删除",
      okButtonProps: { danger: true },
      cancelText: "取消",
      async onOk() {
        await deleteOrgUnit(unit.id);
        await loadTree();
      }
    });
  }

  async function saveMember(values: OrgMemberDrawerValues) {
    if (!memberDrawer.member) return;
    setSaving(true);
    try {
      const member = memberDrawer.member;
      let targetMemberId = member.id;
      if (values.orgUnitId !== member.orgUnit.id) {
        const nextMember = await addOrgMember(member.userId, values.orgUnitId, values.isPrimary);
        targetMemberId = nextMember.id;
        await removeOrgMember(member.id);
      }
      if (values.isPrimary) {
        await setOrgMemberPrimary(targetMemberId);
      }
      await setOrgMemberPositions(targetMemberId, values.positions ?? []);
      setMemberDrawer({ open: false });
      await loadMembers();
      await loadTree();
      message.success("成员已保存");
    } catch (err) {
      message.error(err instanceof Error ? err.message : "成员保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="tenant-org-page">
      <div className="page-heading">
        <div>
          <Typography.Title level={2}>组织管理</Typography.Title>
          <Typography.Text type="secondary">维护当前租户部门、成员归属和部门职务</Typography.Text>
        </div>
        <Button disabled={!canManageOrg} icon={<PlusOutlined />} type="primary" onClick={() => setUnitDrawer({ open: true, mode: "create" })}>
          新建部门
        </Button>
        <Button disabled={!canImport} onClick={() => setImportOpen(true)}>批量导入</Button>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key)}
        items={[
          {
            key: "units",
            label: "组织架构",
            children: (
              <div className="tenant-org-layout">
                <OrgUnitTreePanel
                  units={units}
                  selectedId={selectedUnitId}
                  onSelect={(unit) => setSelectedUnitId(unit.id)}
                  onCreate={(parent) => setUnitDrawer({ open: true, mode: "create", parent })}
                  onEdit={(unit) => setUnitDrawer({ open: true, mode: "edit", unit })}
                  onMove={(unit) => setUnitDrawer({ open: true, mode: "move", unit })}
                  onDelete={confirmDelete}
                  canManage={canManageOrg}
                />
                <section className="tenant-org-detail-panel">
                  {loadingTree ? (
                    <Empty description="加载中" />
                  ) : selectedUnit ? (
                    <>
                      <div className="tenant-org-detail-title">
                        <div className="tenant-org-detail-icon">
                          <ApartmentOutlined />
                        </div>
                        <div className="tenant-org-detail-heading">
                          <Typography.Title level={3}>{selectedUnit.name}</Typography.Title>
                          <Typography.Text type="secondary">{selectedUnit.status === "enabled" ? "可用于成员和策略选择" : "已停用，历史策略和密文保留"}</Typography.Text>
                        </div>
                        <Tooltip title={selectedUnit.status === "enabled" ? "可用于成员和策略选择" : "停用后不可新增成员或被新策略选择"}>
                          <Tag color={selectedUnit.status === "enabled" ? "blue" : "default"}>{selectedUnit.status === "enabled" ? "启用" : "停用"}</Tag>
                        </Tooltip>
                        <div className="tenant-org-title-actions">
                          <Button disabled={!canManageOrg} icon={<EditOutlined />} onClick={() => setUnitDrawer({ open: true, mode: "edit", unit: selectedUnit })}>
                            编辑
                          </Button>
                          <Button disabled={!canManageOrg} icon={<SwapOutlined />} onClick={() => setUnitDrawer({ open: true, mode: "move", unit: selectedUnit })}>
                            移动
                          </Button>
                          <Dropdown
                            disabled={!canManageOrg}
                            menu={{
                              items: [{ key: "delete", icon: <DeleteOutlined />, label: "删除部门", danger: true }],
                              onClick: () => confirmDelete(selectedUnit)
                            }}
                            trigger={["click"]}
                          >
                            <Button disabled={!canManageOrg} icon={<MoreOutlined />} />
                          </Dropdown>
                        </div>
                      </div>
                      <div className="tenant-org-basic-section">
                        <div className="tenant-org-section-title">基本信息</div>
                        <Descriptions column={2} size="small" className="tenant-org-business-descriptions">
                          <Descriptions.Item label="部门名称">{selectedUnit.name}</Descriptions.Item>
                          <Descriptions.Item label="上级部门">{selectedParent?.name ?? "无"}</Descriptions.Item>
                          <Descriptions.Item label="负责人">{selectedLeader?.nickname || selectedLeader?.username || selectedLeader?.email || "未设置"}</Descriptions.Item>
                          <Descriptions.Item label="成员数量">{selectedActiveMemberCount}</Descriptions.Item>
                          <Descriptions.Item label="部门职务">{selectedPositionSummary}</Descriptions.Item>
                          <Descriptions.Item label="状态">{selectedUnit.status === "enabled" ? "启用" : "停用"}</Descriptions.Item>
                        </Descriptions>
                      </div>
                      <Collapse
                        className="tenant-org-advanced"
                        ghost
                        items={[
                          {
                            key: "advanced",
                            label: "高级信息",
                            children: (
                              <Descriptions column={1} size="small">
                                <Descriptions.Item label="稳定编码">{selectedUnit.code}</Descriptions.Item>
                                <Descriptions.Item label="路径">{selectedUnit.path}</Descriptions.Item>
                                <Descriptions.Item label="层级">{selectedUnit.level}</Descriptions.Item>
                                <Descriptions.Item label="排序">{selectedUnit.sortOrder}</Descriptions.Item>
                                <Descriptions.Item label="属性值">{selectedUnit.attributeValue?.valueCode ?? "-"}</Descriptions.Item>
                              </Descriptions>
                            )
                          }
                        ]}
                      />
                    </>
                  ) : (
                    <Empty description="请选择部门" />
                  )}
                </section>
              </div>
            )
          },
          {
            key: "members",
            label: "成员管理",
            children: (
              <OrgMemberTable
                data={members}
                filters={memberFilters}
                loading={loadingMembers}
                units={units}
                onEdit={(member) => setMemberDrawer({ open: true, member })}
                canManage={canManageOrg}
                onFiltersChange={setMemberFilters}
                onPageChange={(page, pageSize) => void loadMembers(page, pageSize)}
                onRefresh={() => void loadMembers()}
              />
            )
          }
        ]}
      />

      <OrgUnitDrawer
        loading={saving}
        mode={unitDrawer.mode}
        open={unitDrawer.open}
        parent={unitDrawer.parent}
        unit={unitDrawer.unit}
        units={units}
        onClose={() => setUnitDrawer({ open: false, mode: "create" })}
        onSubmit={(values) => void saveUnit(values)}
      />
      <OrgMemberDrawer
        loading={saving}
        member={memberDrawer.member}
        open={memberDrawer.open}
        units={units}
        onClose={() => setMemberDrawer({ open: false })}
        onSave={(values) => void saveMember(values)}
      />
      <TenantImportDrawer open={importOpen} type="org_units" onClose={() => setImportOpen(false)} onCompleted={async () => { await loadTree(); if (membersLoaded) await loadMembers(); }} />
    </div>
  );
}

function positionSummary(hasLeader: boolean, deputyCount: number) {
  const parts = [];
  if (hasLeader) parts.push("1 名负责人");
  if (deputyCount > 0) parts.push(`${deputyCount} 名副负责人`);
  return parts.length > 0 ? parts.join(" / ") : "未设置";
}
