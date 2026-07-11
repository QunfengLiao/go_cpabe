-- 009_add_tenant_branding.sql
-- 为租户增加品牌主题配置字段。字段允许为空，Service 会按稳定租户 code 提供默认品牌兜底。

ALTER TABLE tenants
  ADD COLUMN logo_url VARCHAR(512) NULL COMMENT '租户 Logo 静态资源 URL' AFTER description,
  ADD COLUMN login_background_url VARCHAR(512) NULL COMMENT '登录页背景资源 URL' AFTER logo_url,
  ADD COLUMN workspace_background_url VARCHAR(512) NULL COMMENT '工作台背景资源 URL' AFTER login_background_url,
  ADD COLUMN primary_color VARCHAR(32) NULL COMMENT '租户主色 CSS 色值' AFTER workspace_background_url,
  ADD COLUMN sidebar_color VARCHAR(32) NULL COMMENT '侧边栏强调色 CSS 色值' AFTER primary_color,
  ADD COLUMN background_start VARCHAR(32) NULL COMMENT '工作台渐变起始色' AFTER sidebar_color,
  ADD COLUMN background_end VARCHAR(32) NULL COMMENT '工作台渐变结束色' AFTER background_start,
  ADD COLUMN background_glow VARCHAR(32) NULL COMMENT '工作台柔光色' AFTER background_end;

UPDATE tenants
SET
  logo_url = '/tenant-branding/scnu/logo.png',
  login_background_url = '/tenant-branding/scnu/logo.png',
  workspace_background_url = '/tenant-branding/scnu/logo.png',
  primary_color = '#1c5d99',
  sidebar_color = '#1d4f91',
  background_start = '#f7fbff',
  background_end = '#fffaf0',
  background_glow = '#7db7e8'
WHERE code = 'scnu'
  AND logo_url IS NULL;

UPDATE tenants
SET
  logo_url = '/tenant-branding/sangfor/logo.png',
  login_background_url = '/tenant-branding/sangfor/logo.png',
  workspace_background_url = '/tenant-branding/sangfor/logo.png',
  primary_color = '#183b73',
  sidebar_color = '#102a55',
  background_start = '#f3f6fa',
  background_end = '#e8eef6',
  background_glow = '#4f8edb'
WHERE code = 'sangfor'
  AND logo_url IS NULL;

UPDATE tenants
SET
  logo_url = '/tenant-branding/aia/logo.png',
  login_background_url = '/tenant-branding/aia/logo.png',
  workspace_background_url = '/tenant-branding/aia/logo.png',
  primary_color = '#d71920',
  sidebar_color = '#b5121b',
  background_start = '#fffafa',
  background_end = '#f7f8fb',
  background_glow = '#f05a61'
WHERE code IN ('aia', 'aia-hk')
  AND logo_url IS NULL;
