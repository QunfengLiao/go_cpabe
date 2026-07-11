import { Drawer, Form, Input, InputNumber, Select } from "antd";
import { useEffect } from "react";
import type { OrgUnitNode, OrgUnitStatus } from "../../api/tenantOrg";
import { flattenUnits } from "./OrgUnitTreePanel";

export type OrgUnitDrawerMode = "create" | "edit" | "move";

export interface OrgUnitDrawerValues {
  parentId?: number;
  name?: string;
  sortOrder?: number;
  status?: OrgUnitStatus;
}

interface OrgUnitDrawerProps {
  mode: OrgUnitDrawerMode;
  open: boolean;
  unit?: OrgUnitNode;
  parent?: OrgUnitNode;
  units: OrgUnitNode[];
  loading?: boolean;
  onClose: () => void;
  onSubmit: (values: OrgUnitDrawerValues) => void;
}

export function OrgUnitDrawer({ mode, open, unit, parent, units, loading, onClose, onSubmit }: OrgUnitDrawerProps) {
  const [form] = Form.useForm<OrgUnitDrawerValues>();
  const allUnits = flattenUnits(units);
  const moveOptions = allUnits.filter((item) => !unit || (item.id !== unit.id && !item.path.startsWith(`${unit.path}/`)));

  useEffect(() => {
    if (!open) return;
    form.setFieldsValue({
      parentId: mode === "create" ? parent?.id : unit?.parentId,
      name: unit?.name ?? "",
      sortOrder: unit?.sortOrder ?? 0,
      status: unit?.status ?? "enabled"
    });
  }, [form, mode, open, parent?.id, unit]);

  return (
    <Drawer
      destroyOnClose
      open={open}
      title={titleForMode(mode)}
      width={420}
      onClose={onClose}
      extra={null}
      footer={null}
    >
      <Form form={form} layout="vertical" onFinish={onSubmit}>
        {mode !== "edit" && (
          <Form.Item label="父部门" name="parentId">
            <Select
              allowClear
              placeholder="作为根部门"
              options={moveOptions.map((item) => ({ label: item.name, value: item.id }))}
            />
          </Form.Item>
        )}
        {mode !== "move" && (
          <Form.Item label="部门名称" name="name" rules={[{ required: true, message: "请输入部门名称" }]}>
            <Input maxLength={64} />
          </Form.Item>
        )}
        <Form.Item label="排序" name="sortOrder">
          <InputNumber min={0} precision={0} style={{ width: "100%" }} />
        </Form.Item>
        {mode === "edit" && (
          <Form.Item label="状态" name="status">
            <Select
              options={[
                { label: "启用", value: "enabled" },
                { label: "停用", value: "disabled" }
              ]}
            />
          </Form.Item>
        )}
        <div className="drawer-actions">
          <button className="secondary-action" type="button" onClick={onClose}>
            取消
          </button>
          <button className="primary-action" disabled={loading} type="submit">
            保存
          </button>
        </div>
      </Form>
    </Drawer>
  );
}

function titleForMode(mode: OrgUnitDrawerMode) {
  if (mode === "create") return "创建部门";
  if (mode === "move") return "移动部门";
  return "编辑部门";
}
