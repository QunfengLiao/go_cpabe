import { Outlet } from "react-router-dom";
import { ThemeSwitcher } from "./ThemeSwitcher";

const productFeatures = [
  {
    title: "属性访问控制",
    description: "基于用户属性与访问策略完成授权判断。"
  },
  {
    title: "文件加密共享",
    description: "文件加密存储，避免明文数据泄露。"
  },
  {
    title: "双 Token 登录态",
    description: "短期访问令牌配合长期刷新令牌。"
  },
  {
    title: "多账号切换",
    description: "登录过的账号可快速切换，无需重复输入密码。"
  }
];

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
              <strong>{feature.title}</strong>
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
