-- 历史万级用户导入在邮箱唯一键命中其他用户名时，可能继续以 user_id=0 写入关系表。
-- users 主键由 MySQL AUTO_INCREMENT 生成，0 永远不是合法业务用户；因此只按精确零主键清理，
-- 不使用用户名、邮箱或模糊条件，避免误删任何真实账号及其关系。
DELETE FROM user_attributes WHERE user_id = 0;
DELETE FROM tenant_org_member_roles WHERE user_id = 0;
DELETE FROM tenant_org_members WHERE user_id = 0;
DELETE FROM user_roles WHERE user_id = 0;
DELETE FROM tenant_users WHERE user_id = 0;
