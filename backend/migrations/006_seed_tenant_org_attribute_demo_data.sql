-- 演示租户组织架构与属性字典种子数据。
-- 说明：本脚本只依赖 tenants 表中已存在的 code，不创建用户、不修改密码、不授予登录角色。
-- 可重复执行：使用 INSERT ... SELECT ... WHERE NOT EXISTS 保持幂等。

SET @sangfor_id := (SELECT id FROM tenants WHERE code = 'sangfor' LIMIT 1);
SET @scnu_id := (SELECT id FROM tenants WHERE code = 'scnu' LIMIT 1);
SET @aia_id := (SELECT id FROM tenants WHERE code = 'aia-hk' LIMIT 1);

-- 深信服科技组织树
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, NULL, 'SECURITY_BG', '安全 BG', '/SECURITY_BG', 1, 10, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'SECURITY_BG');
SET @sf_security_bg := (SELECT id FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'SECURITY_BG' LIMIT 1);

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_security_bg, 'ENDPOINT_SECURITY', '终端安全产品线', '/SECURITY_BG/ENDPOINT_SECURITY', 2, 10, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'ENDPOINT_SECURITY');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_security_bg, 'NETWORK_SECURITY', '网络安全产品线', '/SECURITY_BG/NETWORK_SECURITY', 2, 20, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'NETWORK_SECURITY');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_security_bg, 'CLOUD_SECURITY', '云安全产品线', '/SECURITY_BG/CLOUD_SECURITY', 2, 30, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'CLOUD_SECURITY');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_security_bg, 'SECURITY_RESEARCH', '安全研究院', '/SECURITY_BG/SECURITY_RESEARCH', 2, 40, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'SECURITY_RESEARCH');

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, NULL, 'CLOUD_BG', '云 BG', '/CLOUD_BG', 1, 20, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'CLOUD_BG');
SET @sf_cloud_bg := (SELECT id FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'CLOUD_BG' LIMIT 1);

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_cloud_bg, 'CLOUD_PLATFORM', '云计算平台部', '/CLOUD_BG/CLOUD_PLATFORM', 2, 10, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'CLOUD_PLATFORM');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_cloud_bg, 'HCI_PRODUCT', '超融合产品部', '/CLOUD_BG/HCI_PRODUCT', 2, 20, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'HCI_PRODUCT');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_cloud_bg, 'CLOUD_NATIVE', '云原生产品部', '/CLOUD_BG/CLOUD_NATIVE', 2, 30, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'CLOUD_NATIVE');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_cloud_bg, 'CLOUD_OPS', '云运维部', '/CLOUD_BG/CLOUD_OPS', 2, 40, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'CLOUD_OPS');

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, NULL, 'AI_BG', 'AI BG', '/AI_BG', 1, 30, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'AI_BG');
SET @sf_ai_bg := (SELECT id FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'AI_BG' LIMIT 1);

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_ai_bg, 'AI_PLATFORM', 'AI 平台部', '/AI_BG/AI_PLATFORM', 2, 10, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'AI_PLATFORM');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_ai_bg, 'AGENT_APP', '智能体应用部', '/AI_BG/AGENT_APP', 2, 20, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'AGENT_APP');
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, @sf_ai_bg, 'DATA_INTELLIGENCE', '数据智能部', '/AI_BG/DATA_INTELLIGENCE', 2, 30, 'enabled'
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @sangfor_id AND code = 'DATA_INTELLIGENCE');

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @sangfor_id, NULL, code, name, CONCAT('/', code), 1, sort_order, 'enabled'
FROM (
  SELECT 'MARKETING' code, '市场体系' name, 40 sort_order UNION ALL
  SELECT 'CUSTOMER_SERVICE', '客户服务体系', 50 UNION ALL
  SELECT 'PROCESS_IT', '流程 IT 部', 60 UNION ALL
  SELECT 'FINANCE_MGMT', '财经管理部', 70 UNION ALL
  SELECT 'PROCUREMENT', '采购部', 80 UNION ALL
  SELECT 'LEGAL', '法务部', 90
) v
WHERE @sangfor_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units u WHERE u.tenant_id = @sangfor_id AND u.code = v.code);

-- 四川师范大学组织树
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @scnu_id, NULL, 'CS_SCHOOL', '计算机科学学院', '/CS_SCHOOL', 1, 10, 'enabled'
WHERE @scnu_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units WHERE tenant_id = @scnu_id AND code = 'CS_SCHOOL');
SET @scnu_cs := (SELECT id FROM tenant_org_units WHERE tenant_id = @scnu_id AND code = 'CS_SCHOOL' LIMIT 1);

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @scnu_id, @scnu_cs, code, name, CONCAT('/CS_SCHOOL/', code), 2, sort_order, 'enabled'
FROM (
  SELECT 'SOFTWARE_ENGINEERING' code, '软件工程系' name, 10 sort_order UNION ALL
  SELECT 'NETWORK_ENGINEERING', '网络工程系', 20 UNION ALL
  SELECT 'AI_LAB', '人工智能实验室', 30
) v
WHERE @scnu_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units u WHERE u.tenant_id = @scnu_id AND u.code = v.code);

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @scnu_id, NULL, code, name, CONCAT('/', code), 1, sort_order, 'enabled'
FROM (
  SELECT 'MATH_SCHOOL' code, '数学科学学院' name, 20 sort_order UNION ALL
  SELECT 'PHYSICS_EE_SCHOOL', '物理与电子工程学院', 30 UNION ALL
  SELECT 'CHEM_MATERIAL_SCHOOL', '化学与材料科学学院', 40 UNION ALL
  SELECT 'ACADEMIC_AFFAIRS', '教务处', 50 UNION ALL
  SELECT 'RESEARCH_OFFICE', '科研处', 60 UNION ALL
  SELECT 'GRADUATE_SCHOOL', '研究生院', 70 UNION ALL
  SELECT 'IT_MANAGEMENT', '信息化建设与管理处', 80 UNION ALL
  SELECT 'FINANCE_OFFICE', '财务处', 90 UNION ALL
  SELECT 'LIBRARY', '图书馆', 100
) v
WHERE @scnu_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units u WHERE u.tenant_id = @scnu_id AND u.code = v.code);

-- 香港友邦保险组织树
INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @aia_id, NULL, code, name, CONCAT('/', code), 1, sort_order, 'enabled'
FROM (
  SELECT 'LIFE_INSURANCE' code, '寿险业务部' name, 10 sort_order UNION ALL
  SELECT 'HEALTH_INSURANCE', '健康险业务部', 20 UNION ALL
  SELECT 'GROUP_INSURANCE', '团体保险部', 30 UNION ALL
  SELECT 'ACTUARIAL', '精算部', 40 UNION ALL
  SELECT 'RISK_MANAGEMENT', '风险管理部', 50 UNION ALL
  SELECT 'CLAIMS_SERVICE', '理赔服务部', 60 UNION ALL
  SELECT 'CUSTOMER_SERVICE', '客户服务部', 70 UNION ALL
  SELECT 'DIGITAL_TECH', '数字化与科技部', 80 UNION ALL
  SELECT 'FINANCE', '财务部', 90 UNION ALL
  SELECT 'COMPLIANCE_LEGAL', '合规法务部', 100 UNION ALL
  SELECT 'CHANNEL_MANAGEMENT', '渠道管理部', 110
) v
WHERE @aia_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units u WHERE u.tenant_id = @aia_id AND u.code = v.code);
SET @aia_digital := (SELECT id FROM tenant_org_units WHERE tenant_id = @aia_id AND code = 'DIGITAL_TECH' LIMIT 1);

INSERT INTO tenant_org_units (tenant_id, parent_id, code, name, path, level, sort_order, status)
SELECT @aia_id, @aia_digital, code, name, CONCAT('/DIGITAL_TECH/', code), 2, sort_order, 'enabled'
FROM (
  SELECT 'DATA_PLATFORM' code, '数据平台部' name, 10 sort_order UNION ALL
  SELECT 'CORE_SYSTEM', '核心系统部', 20 UNION ALL
  SELECT 'INFO_SECURITY', '信息安全部', 30
) v
WHERE @aia_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM tenant_org_units u WHERE u.tenant_id = @aia_id AND u.code = v.code);

-- 为每个演示租户写入属性定义。
INSERT INTO tenant_attributes (tenant_id, attr_code, attr_name, attr_type, value_source, is_required, is_policy_enabled, description, status)
SELECT t.id, v.attr_code, v.attr_name, v.attr_type, v.value_source, v.is_required, v.is_policy_enabled, v.description, 'enabled'
FROM tenants t
JOIN (
  SELECT 'department' attr_code, '部门' attr_name, 'tree' attr_type, 'org_tree' value_source, 1 is_required, 1 is_policy_enabled, '当前租户组织树部门属性' description UNION ALL
  SELECT 'org_role', '部门角色', 'enum', 'manual', 0, 1, '用户在具体部门内的通用角色' UNION ALL
  SELECT 'tenant_role', '租户角色', 'enum', 'derived', 0, 1, '用户在当前租户下的系统角色' UNION ALL
  SELECT 'security_level', '安全等级', 'number', 'manual', 0, 1, '演示用数字安全等级' UNION ALL
  SELECT 'data_category', '数据分类', 'enum', 'manual', 0, 1, '演示数据分类属性'
) v
WHERE t.code IN ('sangfor', 'scnu', 'aia-hk')
  AND NOT EXISTS (SELECT 1 FROM tenant_attributes a WHERE a.tenant_id = t.id AND a.attr_code = v.attr_code);

-- department 属性值来自组织树。
INSERT INTO tenant_attribute_values (tenant_id, attribute_id, value_code, value_label, value_path, org_unit_id, sort_order, status)
SELECT u.tenant_id, a.id, u.code, u.name, u.path, u.id, u.sort_order, 'enabled'
FROM tenant_org_units u
JOIN tenant_attributes a ON a.tenant_id = u.tenant_id AND a.attr_code = 'department'
WHERE u.status = 'enabled'
  AND NOT EXISTS (
    SELECT 1 FROM tenant_attribute_values v
    WHERE v.tenant_id = u.tenant_id AND v.attribute_id = a.id AND v.value_code = u.code
  );

-- 枚举属性值。
INSERT INTO tenant_attribute_values (tenant_id, attribute_id, value_code, value_label, sort_order, status)
SELECT a.tenant_id, a.id, v.value_code, v.value_label, v.sort_order, 'enabled'
FROM tenant_attributes a
JOIN (
  SELECT 'org_role' attr_code, 'ORG_MANAGER' value_code, '部门主管' value_label, 10 sort_order UNION ALL
  SELECT 'org_role', 'ORG_MEMBER', '部门成员', 20 UNION ALL
  SELECT 'org_role', 'DATA_OWNER', '数据拥有者', 30 UNION ALL
  SELECT 'org_role', 'DATA_VISITOR', '数据访问者', 40 UNION ALL
  SELECT 'tenant_role', 'TENANT_ADMIN', '租户管理员', 10 UNION ALL
  SELECT 'tenant_role', 'DATA_OWNER', '数据拥有者', 20 UNION ALL
  SELECT 'tenant_role', 'DATA_VISITOR', '数据访问者', 30 UNION ALL
  SELECT 'tenant_role', 'DO', '数据拥有者（兼容 DO）', 40 UNION ALL
  SELECT 'tenant_role', 'DU', '数据访问者（兼容 DU）', 50 UNION ALL
  SELECT 'data_category', 'PUBLIC', '公开数据', 10 UNION ALL
  SELECT 'data_category', 'INTERNAL', '内部数据', 20 UNION ALL
  SELECT 'data_category', 'CONFIDENTIAL', '敏感数据', 30
) v ON v.attr_code = a.attr_code
WHERE a.tenant_id IN (@sangfor_id, @scnu_id, @aia_id)
  AND NOT EXISTS (
    SELECT 1 FROM tenant_attribute_values existing
    WHERE existing.tenant_id = a.tenant_id AND existing.attribute_id = a.id AND existing.value_code = v.value_code
  );
