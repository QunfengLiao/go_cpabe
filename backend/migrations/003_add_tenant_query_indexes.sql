CREATE INDEX idx_user_roles_user_tenant_role ON user_roles (user_id, tenant_id, role_id);
CREATE INDEX idx_tenant_users_tenant_status_user ON tenant_users (tenant_id, status, user_id);
