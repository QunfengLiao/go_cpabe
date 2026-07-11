import { Checkbox, Input, Space, Tag, Typography } from "antd";
import { useMemo, useState } from "react";
import type { CheckboxChangeEvent } from "antd/es/checkbox";
import type { PermissionDTO } from "../../types";

interface PermissionSelectorProps {
  permissions: PermissionDTO[];
  value: string[];
  disabled?: boolean;
  onChange: (value: string[]) => void;
}

export function PermissionSelector({ permissions, value, disabled, onChange }: PermissionSelectorProps) {
  const [keyword, setKeyword] = useState("");
  const selected = new Set(value);
  const tenantPermissions = useMemo(
    () => permissions.filter((permission) => permission.scopeType === "TENANT" && permission.status === "ACTIVE"),
    [permissions]
  );
  const grouped = useMemo(() => {
    const query = keyword.trim().toLowerCase();
    return tenantPermissions
      .filter((permission) => {
        if (!query) return true;
        return `${permission.code} ${permission.name} ${permission.description ?? ""}`.toLowerCase().includes(query);
      })
      .reduce<Record<string, PermissionDTO[]>>((groups, permission) => {
        const key = permission.resourceType || "未分组";
        groups[key] ??= [];
        groups[key].push(permission);
        return groups;
      }, {});
  }, [keyword, tenantPermissions]);

  function setGroup(resourceType: string, checked: boolean) {
    const next = new Set(selected);
    for (const permission of grouped[resourceType] ?? []) {
      if (checked) next.add(permission.code);
      else next.delete(permission.code);
    }
    onChange(Array.from(next));
  }

  function setPermission(code: string, checked: boolean) {
    const next = new Set(selected);
    if (checked) next.add(code);
    else next.delete(code);
    onChange(Array.from(next));
  }

  return (
    <div className="permission-selector">
      <div className="permission-selector-toolbar">
        <Input.Search allowClear placeholder="搜索权限名称或 code" value={keyword} onChange={(event) => setKeyword(event.target.value)} />
        <Tag color="blue">已选 {value.length}</Tag>
      </div>
      <div className="permission-selector-groups">
        {Object.entries(grouped).map(([resourceType, items]) => {
          const checkedCount = items.filter((permission) => selected.has(permission.code)).length;
          const allChecked = checkedCount === items.length && items.length > 0;
          const indeterminate = checkedCount > 0 && checkedCount < items.length;
          return (
            <section className="permission-selector-group" key={resourceType}>
              <div className="permission-selector-group-title">
                <Checkbox
                  checked={allChecked}
                  disabled={disabled}
                  indeterminate={indeterminate}
                  onChange={(event: CheckboxChangeEvent) => setGroup(resourceType, event.target.checked)}
                >
                  {resourceType}
                </Checkbox>
                <Typography.Text type="secondary">{checkedCount}/{items.length}</Typography.Text>
              </div>
              <div className="permission-selector-items">
                {items.map((permission) => (
                  <Checkbox
                    checked={selected.has(permission.code)}
                    disabled={disabled}
                    key={permission.code}
                    onChange={(event: CheckboxChangeEvent) => setPermission(permission.code, event.target.checked)}
                  >
                    <Space direction="vertical" size={1}>
                      <Typography.Text strong>{permission.name || permission.code}</Typography.Text>
                      <Typography.Text type="secondary">{permission.code}</Typography.Text>
                    </Space>
                  </Checkbox>
                ))}
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}

