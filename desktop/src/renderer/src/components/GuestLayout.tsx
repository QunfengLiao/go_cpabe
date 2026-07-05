import { Outlet } from "react-router-dom";

export function GuestLayout() {
  return (
    <main className="guest-shell">
      {/* 游客态不展示应用侧边栏，是为了把“账号进入系统”和“登录后工作台”两种任务分开。 */}
      <section className="guest-brand-panel" aria-label="系统介绍">
        <div className="guest-brand-mark">CP</div>
        <span className="auth-kicker">CP-ABE File Sharing</span>
        <h1>CP-ABE 加密文件共享系统</h1>
        <p>面向细粒度访问控制场景，支持数据拥有者加密共享文件，数据访问者按属性授权解密访问。</p>
        <div className="guest-feature-list">
          <span>属性访问控制</span>
          <span>文件加密共享</span>
          <span>双 Token 登录态</span>
          <span>资料与头像闭环</span>
        </div>
      </section>
      <section className="guest-form-panel">
        <Outlet />
      </section>
    </main>
  );
}
