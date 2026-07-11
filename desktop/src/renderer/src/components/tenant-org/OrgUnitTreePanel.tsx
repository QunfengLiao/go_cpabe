import { useMemo, useState } from "react";
import { Button, Dropdown, Empty, Input, Tag, Typography, type MenuProps } from "antd";
import { DeleteOutlined, DownOutlined, EditOutlined, MoreOutlined, PlusOutlined, RightOutlined, SearchOutlined, SwapOutlined } from "@ant-design/icons";
import type { OrgUnitNode } from "../../api/tenantOrg";

interface OrgUnitTreePanelProps {
  units: OrgUnitNode[];
  selectedId?: number;
  onSelect: (unit: OrgUnitNode) => void;
  onCreate: (parent?: OrgUnitNode) => void;
  onEdit: (unit: OrgUnitNode) => void;
  onMove: (unit: OrgUnitNode) => void;
  onDelete: (unit: OrgUnitNode) => void;
  canManage?: boolean;
}

export function OrgUnitTreePanel({ units, selectedId, onSelect, onCreate, onEdit, onMove, onDelete, canManage = true }: OrgUnitTreePanelProps) {
  const [keyword, setKeyword] = useState("");
  const [collapsedIds, setCollapsedIds] = useState<Set<number>>(() => new Set());
  const visibleUnits = useMemo(() => filterUnits(units, keyword), [keyword, units]);
  const flatUnits = useMemo(() => flattenUnits(units), [units]);
  const searchActive = keyword.trim().length > 0;

  function toggleUnit(unitId: number) {
    setCollapsedIds((current) => {
      const next = new Set(current);
      if (next.has(unitId)) {
        next.delete(unitId);
      } else {
        next.add(unitId);
      }
      return next;
    });
  }

  return (
    <section className="tenant-org-tree-panel">
      <div className="tenant-org-panel-header">
        <Typography.Title level={4}>部门树</Typography.Title>
        <Typography.Text type="secondary">{flatUnits.length} 个部门</Typography.Text>
      </div>
      <Input
        allowClear
        className="tenant-org-tree-search"
        prefix={<SearchOutlined />}
        placeholder="搜索部门"
        value={keyword}
        onChange={(event) => setKeyword(event.target.value)}
      />
      {visibleUnits.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无部门" />
      ) : (
        <div className="tenant-org-tree-list" role="tree">
          {visibleUnits.map((unit) => (
            <OrgUnitTreeNode
              collapsedIds={collapsedIds}
              key={unit.id}
              searchActive={searchActive}
              selectedId={selectedId}
              unit={unit}
              onCreate={onCreate}
              onDelete={onDelete}
              onEdit={onEdit}
              onMove={onMove}
              onSelect={onSelect}
              onToggle={toggleUnit}
              canManage={canManage}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function OrgUnitTreeNode({
  unit,
  selectedId,
  collapsedIds,
  searchActive,
  onSelect,
  onCreate,
  onEdit,
  onMove,
  onDelete,
  onToggle,
  canManage
}: {
  unit: OrgUnitNode;
  selectedId?: number;
  collapsedIds: Set<number>;
  searchActive: boolean;
  onSelect: (unit: OrgUnitNode) => void;
  onCreate: (parent?: OrgUnitNode) => void;
  onEdit: (unit: OrgUnitNode) => void;
  onMove: (unit: OrgUnitNode) => void;
  onDelete: (unit: OrgUnitNode) => void;
  onToggle: (unitId: number) => void;
  canManage: boolean;
}) {
  const hasChildren = unit.children.length > 0;
  const expanded = searchActive || !collapsedIds.has(unit.id);
  const selected = selectedId === unit.id;
  const menu: MenuProps["items"] = [
    { key: "child", icon: <PlusOutlined />, label: "新建子部门" },
    { key: "edit", icon: <EditOutlined />, label: "编辑" },
    { key: "move", icon: <SwapOutlined />, label: "移动" },
    { key: "delete", icon: <DeleteOutlined />, label: "删除", danger: true }
  ];

  return (
    <div className="tenant-org-tree-branch" role="treeitem" aria-expanded={hasChildren ? expanded : undefined} aria-selected={selected}>
      <div className={selected ? "tenant-org-tree-node tenant-org-tree-node-selected" : "tenant-org-tree-node"} onClick={() => onSelect(unit)}>
        <button
          aria-label={expanded ? "收起部门" : "展开部门"}
          className={hasChildren ? "tenant-org-tree-toggle" : "tenant-org-tree-toggle tenant-org-tree-toggle-placeholder"}
          disabled={!hasChildren}
          onClick={(event) => {
            event.stopPropagation();
            if (hasChildren) onToggle(unit.id);
          }}
          type="button"
        >
          {hasChildren ? expanded ? <DownOutlined /> : <RightOutlined /> : null}
        </button>
        <span className="tenant-org-tree-label">{unit.name}</span>
        {unit.status === "disabled" && <Tag>停用</Tag>}
        {canManage && (
          <Dropdown
            menu={{
              items: menu,
              onClick: ({ key, domEvent }) => {
                domEvent.stopPropagation();
                if (key === "child") onCreate(unit);
                if (key === "edit") onEdit(unit);
                if (key === "move") onMove(unit);
                if (key === "delete") onDelete(unit);
              }
            }}
            trigger={["click"]}
          >
            <Button className="tenant-org-tree-action" icon={<MoreOutlined />} size="small" type="text" onClick={(event) => event.stopPropagation()} />
          </Dropdown>
        )}
      </div>
      {hasChildren && expanded && (
        <div className="tenant-org-tree-children" role="group">
          {unit.children.map((child) => (
            <OrgUnitTreeNode
              collapsedIds={collapsedIds}
              key={child.id}
              searchActive={searchActive}
              selectedId={selectedId}
              unit={child}
              onCreate={onCreate}
              onDelete={onDelete}
              onEdit={onEdit}
              onMove={onMove}
              onSelect={onSelect}
              onToggle={onToggle}
              canManage={canManage}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function flattenUnits(units: OrgUnitNode[]): OrgUnitNode[] {
  return units.flatMap((unit) => [unit, ...flattenUnits(unit.children)]);
}

function filterUnits(units: OrgUnitNode[], keyword: string): OrgUnitNode[] {
  const query = keyword.trim().toLowerCase();
  if (!query) return units;
  return units.flatMap((unit) => {
    const children = filterUnits(unit.children, query);
    if (unit.name.toLowerCase().includes(query) || children.length > 0) {
      return [{ ...unit, children }];
    }
    return [];
  });
}
