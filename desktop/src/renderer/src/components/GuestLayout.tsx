import type { ReactNode } from "react";
import { Outlet } from "react-router-dom";
import { ThemeSwitcher } from "./ThemeSwitcher";

const productFeatures = [
  {
    icon: "fileLock",
    title: "策略加密共享",
    description: "基于 CP-ABE 对文件进行策略加密，满足访问条件的用户才可解密。"
  },
  {
    icon: "workflow",
    title: "访问树可视化",
    description: "支持将策略表达式转换为访问树，直观展示 AND / OR 阈值结构。"
  },
  {
    icon: "chart",
    title: "多方案对比",
    description: "集成 CP-ABE、RSA+AES 等方案，对比加密、解密和密文开销。"
  },
  {
    icon: "key",
    title: "密钥权限管理",
    description: "围绕用户属性、私钥分发和访问策略，完成细粒度权限控制。"
  }
] as const;

type FeatureIconName = (typeof productFeatures)[number]["icon"];

function FeatureIcon({ name }: { name: FeatureIconName }) {
  const paths: Record<FeatureIconName, ReactNode> = {
    fileLock: (
      <>
        <path d="M7 3h6l4 4v14H7V3Z" />
        <path d="M13 3v5h5" />
        <path d="M9.5 15.5h5" />
        <path d="M10 15.5v-2a2 2 0 0 1 4 0v2" />
      </>
    ),
    workflow: (
      <>
        <circle cx="6" cy="6" r="2.5" />
        <circle cx="18" cy="6" r="2.5" />
        <circle cx="12" cy="18" r="2.5" />
        <path d="M7.8 8.1 11 15.5" />
        <path d="m16.2 8.1-3.2 7.4" />
        <path d="M8.5 6h7" />
      </>
    ),
    chart: (
      <>
        <path d="M4 20h16" />
        <path d="M7 16v-5" />
        <path d="M12 16V7" />
        <path d="M17 16v-8" />
        <path d="M6 11h2" />
        <path d="M11 7h2" />
        <path d="M16 8h2" />
      </>
    ),
    key: (
      <>
        <circle cx="8" cy="12" r="3.5" />
        <path d="M11.5 12H21" />
        <path d="M17 12v3" />
        <path d="M14.5 12v2" />
      </>
    )
  };

  return (
    <svg className="guest-feature-icon" viewBox="0 0 24 24" aria-hidden="true">
      {paths[name]}
    </svg>
  );
}

export function GuestLayout() {
  return (
    <main className="guest-shell">
      <div className="guest-appearance">
        <ThemeSwitcher />
      </div>
      {/* 游客态不展示应用侧边栏，是为了把“账号进入系统”和“登录后工作台”两种任务分开。 */}
      <section className="guest-brand-panel" aria-label="系统介绍">
        <div className="guest-brand-stack">
          <div className="guest-brand-identity">
            <div className="guest-brand-mark" aria-hidden="true">
              <span className="guest-brand-lock" />
            </div>
            <div className="guest-brand-name">
              <strong>CP-ABE</strong>
              <span>加密文件共享系统</span>
            </div>
          </div>
          <div className="guest-brand-copy">
            <h1>CP-ABE 加密文件共享系统</h1>
            <p>面向细粒度访问控制场景，支持数据拥有者加密共享文件，数据访问者按属性授权解密访问。</p>
          </div>
        </div>
        <div className="guest-feature-list">
          {productFeatures.map((feature) => (
            <article className="guest-feature-card" key={feature.title}>
              <strong>
                <FeatureIcon name={feature.icon} />
                {feature.title}
              </strong>
              <span>{feature.description}</span>
            </article>
          ))}
        </div>
      </section>
      <section className="guest-form-panel">
        <Outlet />
      </section>
    </main>
  );
}
