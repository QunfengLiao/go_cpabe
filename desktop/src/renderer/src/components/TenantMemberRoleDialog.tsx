import { Alert, Button, Checkbox, Spin, Tag, Typography } from "antd";
import { useEffect, useMemo, useState } from "react";
import { filterTenantVisibleRoles, getTenantMemberRoles, listTenantRoles, rbacErrorMessage, replaceTenantMemberRoles } from "../api/rbac";
import { useAuth } from "../auth/AuthContext";
import type { MemberRoleDTO, TenantMember, TenantRole, TenantRoleDTO } from "../types";
import { categoryLabel, normalizeCategory } from "./tenant-rbac/RoleList";

interface TenantMemberRoleDialogProps {
  member: TenantMember;
  onClose: () => void;
  onSaved: (result: MemberRoleDTO) => void;
}

export function TenantMemberRoleDialog({ member, onClose, onSaved }: TenantMemberRoleDialogProps) {
  const auth = useAuth();
  const [roles, setRoles] = useState<TenantRoleDTO[]>([]);
  const [currentRoles, setCurrentRoles] = useState<TenantRoleDTO[]>([]);
  const [selectedRoleCodes, setSelectedRoleCodes] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const currentRoleCodes = useMemo(() => new Set(currentRoles.map((role) => role.code)), [currentRoles]);
  const tenantId = Number(auth.currentTenantId) || undefined;
  const groupedRoles = useMemo(() => {
    return filterTenantVisibleRoles(roles, tenantId)
      .reduce<Record<string, TenantRoleDTO[]>>((groups, role) => {
        const key = normalizeCategory(role);
        groups[key] ??= [];
        groups[key].push(role);
        return groups;
      }, {});
  }, [roles, tenantId]);

  useEffect(() => {
    let alive = true;
    async function load() {
      setLoading(true);
      setError("");
      try {
        const [roleItems, memberRoles] = await Promise.all([listTenantRoles(), getTenantMemberRoles(member.user_id)]);
        if (!alive) return;
        setRoles(roleItems);
        setCurrentRoles(memberRoles.roles);
        setSelectedRoleCodes(memberRoles.roles.map((role) => role.code));
      } catch (err) {
        if (alive) setError(rbacErrorMessage(err, "成员角色加载失败"));
      } finally {
        if (alive) setLoading(false);
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [member.user_id]);

  function toggleRole(role: TenantRoleDTO, checked: boolean) {
    setSelectedRoleCodes((current) => {
      const next = new Set(current);
      if (checked) next.add(role.code);
      else next.delete(role.code);
      return Array.from(next);
    });
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      const result = await replaceTenantMemberRoles(member.user_id, selectedRoleCodes);
      onSaved(result);
    } catch (err) {
      setError(rbacErrorMessage(err, "成员角色保存失败"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="role-dialog-backdrop" role="presentation">
      <div className="role-dialog role-dialog-wide" role="dialog" aria-modal="true" aria-labelledby="tenant-member-role-title">
        <div>
          <h3 id="tenant-member-role-title">分配角色</h3>
          <p>为当前租户成员设置一个或多个角色，保存时提交完整角色集合。</p>
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
        {error && <Alert message={error} type="error" showIcon />}
        {loading ? (
          <div className="role-dialog-loading"><Spin /> 正在加载角色...</div>
        ) : (
          <div className="role-dialog-role-groups">
            {Object.entries(groupedRoles).map(([category, items]) => (
              <section className="role-dialog-role-group" key={category}>
                <Typography.Text strong>{categoryLabel(category as ReturnType<typeof normalizeCategory>)}</Typography.Text>
                <div className="role-dialog-checkbox-grid">
                  {items.map((role) => {
                    const disabled = role.status === "DISABLED";
                    const checked = selectedRoleCodes.includes(role.code);
                    const changed = checked !== currentRoleCodes.has(role.code);
                    return (
                      <label className={`role-dialog-checkbox${disabled ? " role-dialog-checkbox-disabled" : ""}`} key={role.id}>
                        <Checkbox checked={checked} disabled={disabled || saving} onChange={(event) => toggleRole(role, event.target.checked)} />
                        <span>
                          <strong>{role.name}</strong>
                          <small>{role.code}</small>
                          <em>{role.description || "暂无描述"}</em>
                        </span>
                        {changed && <Tag color={checked ? "green" : "red"}>{checked ? "新增" : "移除"}</Tag>}
                        {disabled && <Tag>已禁用</Tag>}
                      </label>
                    );
                  })}
                </div>
              </section>
            ))}
          </div>
        )}
        <div className="role-dialog-actions">
          <Button disabled={saving} onClick={onClose}>
            取消
          </Button>
          <Button disabled={loading} loading={saving} type="primary" onClick={() => void save()}>
            保存
          </Button>
        </div>
      </div>
    </div>
  );
}

function roleListLabel(roles: TenantRole[]): string {
  if (!roles.length) return "未分配";
  return roles.map(roleDisplayName).join(" / ");
}

function roleDisplayName(role: TenantRole): string {
  if (role === "TENANT_ADMIN") return "租户管理员";
  if (role === "DO") return "数据拥有者 DO";
  if (role === "DU") return "数据使用者 DU";
  return role;
}
