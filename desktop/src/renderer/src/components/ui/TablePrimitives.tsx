import { Button, Dropdown, Table, Tooltip, type MenuProps, type TableProps } from "antd";
import { MoreOutlined } from "@ant-design/icons";

export function DataTable<RecordType extends object>({ className = "", ...props }: TableProps<RecordType>) {
  return <Table {...props} className={`data-table ${className}`.trim()} />;
}

export function RowActions({ primary, detail, menu, loading = false, disabled = false }: { primary?: React.ReactNode; detail?: () => void; menu?: MenuProps["items"]; loading?: boolean; disabled?: boolean }) {
  return (
    <div className="row-actions">
      {primary}
      {detail && <Tooltip title="查看详情"><Button type="text" size="small" onClick={detail}>详情</Button></Tooltip>}
      {menu && <Dropdown menu={{ items: menu }} trigger={["click"]} placement="bottomRight"><Button aria-label="更多操作" type="text" size="small" icon={<MoreOutlined />} loading={loading} disabled={disabled} /></Dropdown>}
    </div>
  );
}
