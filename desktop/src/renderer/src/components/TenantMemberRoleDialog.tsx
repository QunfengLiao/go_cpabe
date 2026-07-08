import type { TenantBusinessRole, TenantMember, TenantRole } from "../types";

interface TenantMemberRoleDialogProps {
  member: TenantMember;
  saving: boolean;
  onClose: () => void;
  onSave: (role: TenantBusinessRole) => void;
}

const businessRoleOptions: Array<{ label: string; value: TenantBusinessRole; internal: TenantRole }> = [
  { label: "数据拥有者", value: "DATA_OWNER", internal: "DO" },
  { label: "数据访问者", value: "DATA_VISITOR", internal: "DU" }
];

export function TenantMemberRoleDialog({ member, saving, onClose, onSave }: TenantMemberRoleDialogProps) {
  const current = businessRoleOptions.find((option) => member.roles.includes(option.internal))?.value ?? "DATA_VISITOR";

  return (
    <div className="role-dialog-backdrop" role="presentation">
      <div className="role-dialog" role="dialog" aria-modal="true" aria-labelledby="tenant-member-role-title">
        <div>
          <h3 id="tenant-member-role-title">分配角色</h3>
          <p>为当前租户成员设置一个普通业务角色。</p>
        </div>
        <dl className="role-dialog-member">
          <div>
            <dt>昵称</dt>
            <dd>{member.nickname || "未命名用户"}</dd>
          </div>
          <div>
            <dt>邮箱</dt>
            <dd>{member.email}</dd>
          </div>
          <div>
            <dt>当前角色</dt>
            <dd>{roleListLabel(member.roles)}</dd>
          </div>
        </dl>
        <div className="role-dialog-options" role="radiogroup" aria-label="普通业务角色">
          {businessRoleOptions.map((option) => (
            <button
              className={`role-option${current === option.value ? " role-option-active" : ""}`}
              disabled={saving}
              key={option.value}
              onClick={() => onSave(option.value)}
              type="button"
            >
              <strong>{option.label}</strong>
              <span>{option.value}</span>
            </button>
          ))}
        </div>
        <div className="role-dialog-actions">
          <button className="secondary-action" disabled={saving} onClick={onClose} type="button">
            取消
          </button>
        </div>
      </div>
    </div>
  );
}

function roleListLabel(roles: TenantRole[]): string {
  if (roles.includes("TENANT_ADMIN")) return "租户管理员";
  if (roles.includes("DO")) return "数据拥有者";
  if (roles.includes("DU")) return "数据访问者";
  return "未分配";
}
