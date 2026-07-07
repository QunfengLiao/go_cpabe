import type { CSSProperties, ReactNode } from "react";
import { commonTenantLoginConfig, getTenantLoginConfig } from "../config/tenantLoginConfigs";

interface TenantBrandPanelProps {
  tenantCode?: string;
}

const featureIcons = {
  tenant: (
    <>
      <rect x="4" y="5" width="16" height="14" rx="2" />
      <path d="M8 9h8" />
      <path d="M8 13h5" />
      <path d="M8 17h3" />
    </>
  ),
  rbac: (
    <>
      <circle cx="8" cy="8" r="3" />
      <circle cx="16" cy="16" r="3" />
      <path d="M10.5 10.5 13.5 13.5" />
      <path d="M16 6v4" />
      <path d="M14 8h4" />
    </>
  ),
  cipher: (
    <>
      <path d="M7 11V8a5 5 0 0 1 10 0v3" />
      <rect x="5" y="11" width="14" height="9" rx="2" />
      <path d="M12 15v2" />
    </>
  ),
  policy: (
    <>
      <circle cx="6" cy="6" r="2.5" />
      <circle cx="18" cy="6" r="2.5" />
      <circle cx="12" cy="18" r="2.5" />
      <path d="M7.8 8.1 11 15.5" />
      <path d="m16.2 8.1-3.2 7.4" />
    </>
  )
} satisfies Record<string, ReactNode>;

const features = [
  { icon: "tenant", title: "租户数据隔离", description: "以 tenant_id 建立组织边界，避免跨租户访问业务数据。" },
  { icon: "rbac", title: "租户内角色", description: "同一账号可在不同租户下拥有不同角色和菜单范围。" },
  { icon: "cipher", title: "密文访问控制", description: "文件以密文共享，解密能力仍由属性私钥和访问策略决定。" },
  { icon: "policy", title: "策略可解释", description: "访问策略、属性和审计记录为后续 CP-ABE 闭环预留上下文。" }
] as const;

export function TenantBrandPanel({ tenantCode }: TenantBrandPanelProps) {
  const config = getTenantLoginConfig(tenantCode) ?? commonTenantLoginConfig;
  const style = {
    "--tenant-accent": config.accent,
    "--tenant-accent-strong": config.accentStrong,
    "--tenant-accent-soft": config.accentSoft,
    "--tenant-accent-text": config.accentText
  } as CSSProperties;

  return (
    <section className={`guest-brand-panel tenant-brand-${config.styleName}`} aria-label="系统介绍" style={style}>
      <div className="guest-brand-stack">
        <div className="guest-brand-identity">
          <div className="guest-brand-mark" aria-hidden="true">
            <span className="guest-brand-lock" />
          </div>
          <div className="guest-brand-name">
            <strong>{config.shortName}</strong>
            <span>{config.name}</span>
          </div>
        </div>
        <div className="guest-brand-copy">
          <h1>{config.title}</h1>
          <p>{config.subtitle}</p>
        </div>
        <div className="tenant-brand-tags" aria-label="能力标签">
          {config.highlights.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </div>
      <div className="guest-feature-list">
        {features.map((feature) => (
          <article className="guest-feature-card" key={feature.title}>
            <strong>
              <svg className="guest-feature-icon" viewBox="0 0 24 24" aria-hidden="true">
                {featureIcons[feature.icon]}
              </svg>
              {feature.title}
            </strong>
            <span>{feature.description}</span>
          </article>
        ))}
      </div>
    </section>
  );
}
