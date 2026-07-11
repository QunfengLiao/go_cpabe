import { Drawer, Form, Select, Switch, Typography } from "antd";
import { useEffect } from "react";
import type { OrgMember, OrgPosition, OrgUnitNode } from "../../api/tenantOrg";
import { flattenUnits } from "./OrgUnitTreePanel";

interface OrgMemberDrawerProps {
  open: boolean;
  member?: OrgMember;
  units: OrgUnitNode[];
  loading?: boolean;
  onClose: () => void;
  onSave: (values: OrgMemberDrawerValues) => void;
}

export interface OrgMemberDrawerValues {
  orgUnitId: number;
  isPrimary: boolean;
  positions: OrgPosition[];
}

export function OrgMemberDrawer({ open, member, units, loading, onClose, onSave }: OrgMemberDrawerProps) {
  const [form] = Form.useForm<OrgMemberDrawerValues>();
  const unitOptions = flattenUnits(units).filter((unit) => unit.status === "enabled").map((unit) => ({ label: unit.name, value: unit.id }));

  useEffect(() => {
    if (!open || !member) return;
    form.setFieldsValue({
      orgUnitId: member.orgUnit.id,
      isPrimary: member.isPrimary,
      positions: member.positions
    });
  }, [form, member, open]);

  return (
    <Drawer destroyOnClose open={open} title="编辑成员" width={440} onClose={onClose}>
      {member && (
        <div className="member-drawer-summary">
          <Typography.Text strong>{member.nickname || member.username || member.email}</Typography.Text>
          <Typography.Text type="secondary">{member.email}</Typography.Text>
        </div>
      )}
      <Form form={form} layout="vertical" onFinish={onSave}>
        <Form.Item label="部门" name="orgUnitId" rules={[{ required: true, message: "请选择部门" }]}>
          <Select options={unitOptions} />
        </Form.Item>
        <Form.Item label="主部门" name="isPrimary" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label="部门职务" name="positions">
          <Select
            allowClear
            mode="multiple"
            options={[
              { label: "部门负责人", value: "ORG_LEADER" },
              { label: "部门副负责人", value: "DEPUTY_LEADER" }
            ]}
          />
        </Form.Item>
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
