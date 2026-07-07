export interface TenantLoginConfig {
  code: string;
  name: string;
  shortName: string;
  title: string;
  subtitle: string;
  buttonText: string;
  styleName: "scnu" | "sangfor" | "aia";
  accent: string;
  accentStrong: string;
  accentSoft: string;
  accentText: string;
  highlights: string[];
}

export const tenantLoginConfigs: Record<string, TenantLoginConfig> = {
  scnu: {
    code: "scnu",
    name: "四川师范大学",
    shortName: "SCNU",
    title: "CP-ABE 科研数据安全共享平台",
    subtitle: "面向课题组、实验室和科研协作场景",
    buttonText: "进入科研数据空间",
    styleName: "scnu",
    accent: "#0ea5e9",
    accentStrong: "#075985",
    accentSoft: "#e0f7ff",
    accentText: "#0f4f74",
    highlights: ["科研协作", "实验数据隔离", "属性密钥授权", "密文策略共享"]
  },
  sangfor: {
    code: "sangfor",
    name: "深信服科技",
    shortName: "SANGFOR",
    title: "企业级密文数据共享与访问控制平台",
    subtitle: "面向企业研发与数据安全协作场景",
    buttonText: "进入企业安全空间",
    styleName: "sangfor",
    accent: "#2563eb",
    accentStrong: "#102a72",
    accentSoft: "#e8f0ff",
    accentText: "#17377f",
    highlights: ["云安全协作", "研发数据保护", "租户级隔离", "RBAC 接口边界"]
  },
  "aia-hk": {
    code: "aia-hk",
    name: "香港友邦保险",
    shortName: "AIA HK",
    title: "金融保险数据安全访问平台",
    subtitle: "面向保险业务和跨部门数据协作场景",
    buttonText: "进入保险数据空间",
    styleName: "aia",
    accent: "#b91c1c",
    accentStrong: "#6f1010",
    accentSoft: "#fff3d6",
    accentText: "#7f1d1d",
    highlights: ["客户数据保护", "合规访问审计", "跨部门协作", "密文访问控制"]
  }
};

export const commonTenantLoginConfig: TenantLoginConfig = {
  code: "",
  name: "通用入口",
  shortName: "CP-ABE",
  title: "CP-ABE 加密文件共享系统",
  subtitle: "面向多租户、RBAC 与密文策略访问控制的统一登录入口",
  buttonText: "进入加密共享工作台",
  styleName: "scnu",
  accent: "#1e679f",
  accentStrong: "#123f72",
  accentSoft: "#eaf5ff",
  accentText: "#23598f",
  highlights: ["多租户隔离", "RBAC 菜单权限", "CP-ABE 解密授权", "审计可追踪"]
};

export function getTenantLoginConfig(code?: string | null): TenantLoginConfig | null {
  if (!code) return commonTenantLoginConfig;
  return tenantLoginConfigs[code] ?? null;
}

export function isKnownTenantCode(code?: string | null): boolean {
  return Boolean(code && tenantLoginConfigs[code]);
}
