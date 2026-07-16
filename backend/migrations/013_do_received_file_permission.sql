-- 013_do_received_file_permission.sql
-- 修复已执行 010 的开发库：DO 也可以进入“分享给我”流程。
-- 该历史权限记录继续保留以兼容旧租户配置，但当前解密材料、密文下载和本地解密不再依赖它；
-- RBAC 只控制文件可见性及管理动作，实际能否解密由密钥信封和本地私钥决定。

INSERT IGNORE INTO role_permissions (role_id, permission_id, granted_by, created_at)
SELECT r.id, p.id, NULL, CURRENT_TIMESTAMP(3)
FROM roles r
JOIN permissions p ON p.code = 'file.decrypt.invoke'
WHERE r.tenant_id = 0 AND r.code = 'DO';
