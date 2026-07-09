import React from "react";
import { createRoot } from "react-dom/client";
import { HashRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { GuestOnly } from "./auth/GuestOnly";
import { RequireAuth } from "./auth/RequireAuth";
import { RequirePlatformAdmin } from "./auth/RequirePlatformAdmin";
import { RequireTenantRole } from "./auth/RequireTenantRole";
import { AppLayout } from "./components/AppLayout";
import { GuestLayout } from "./components/GuestLayout";
import { AccessPolicyBuilderPage } from "./pages/AccessPolicyBuilderPage";
import { AccessPolicyEditorPage } from "./pages/AccessPolicyEditorPage";
import { LoginPage } from "./pages/LoginPage";
import { MyAccessPoliciesPage } from "./pages/MyAccessPoliciesPage";
import { PlatformDashboardPage } from "./pages/PlatformDashboardPage";
import { PlatformPolicyManagementPage } from "./pages/PlatformPolicyManagementPage";
import { PlatformTenantCreatePage } from "./pages/PlatformTenantCreatePage";
import { PlatformTenantDetailPage } from "./pages/PlatformTenantDetailPage";
import { PlatformTenantListPage } from "./pages/PlatformTenantListPage";
import { PlatformTenantUsersPage } from "./pages/PlatformTenantUsersPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RegisterPage } from "./pages/RegisterPage";
import { SelectTenantPage } from "./pages/SelectTenantPage";
import { StartupRedirect } from "./pages/StartupRedirect";
import { TenantAccessPolicyViewPage } from "./pages/TenantAccessPolicyViewPage";
import { TenantMembersPage } from "./pages/TenantMembersPage";
import { prepareStartupTenant } from "./tenantStartup";
import { ThemeProvider } from "./theme/ThemeProvider";
import "./styles.css";

prepareStartupTenant();

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ThemeProvider>
      <HashRouter>
        <AuthProvider>
          <Routes>
            <Route index element={<StartupRedirect />} />
            <Route path="/select-tenant" element={<SelectTenantPage />} />
            {/* 认证页和应用页使用不同布局，避免游客入口在登录后仍占据主要导航。 */}
            <Route element={<GuestOnly />}>
              <Route element={<GuestLayout />}>
                <Route path="/login" element={<LoginPage />} />
                <Route path="/login/:tenantCode" element={<LoginPage />} />
                <Route path="/register" element={<RegisterPage />} />
              </Route>
            </Route>
            <Route element={<RequireAuth />}>
              <Route element={<AppLayout />}>
                <Route path="/profile" element={<ProfilePage />} />
                <Route path="/profile/edit" element={<ProfilePage />} />
                <Route path="/tenant/members" element={<TenantMembersPage />} />
                <Route element={<RequirePlatformAdmin />}>
                  <Route path="/platform" element={<PlatformDashboardPage />} />
                  <Route path="/platform/tenants" element={<PlatformTenantListPage />} />
                  <Route path="/platform/tenants/new" element={<PlatformTenantCreatePage />} />
                  <Route path="/platform/tenants/:tenantId" element={<PlatformTenantDetailPage />} />
                  <Route path="/platform/tenants/:tenantId/users" element={<PlatformTenantUsersPage />} />
                  <Route path="/platform/policies" element={<PlatformPolicyManagementPage />} />
                </Route>
                <Route element={<RequireTenantRole roles={["DO"]} />}>
                  <Route path="/access-policies/builder" element={<AccessPolicyBuilderPage />} />
                  <Route path="/access-policies/builder/editor" element={<AccessPolicyEditorPage />} />
                  <Route path="/access-policies" element={<MyAccessPoliciesPage />} />
                  <Route path="/access-policies/:policyId/edit" element={<AccessPolicyBuilderPage />} />
                  <Route path="/access-policies/:policyId/edit/tree" element={<AccessPolicyEditorPage />} />
                </Route>
                <Route element={<RequireTenantRole roles={["TENANT_ADMIN"]} />}>
                  <Route path="/tenant/access-policies" element={<TenantAccessPolicyViewPage />} />
                </Route>
                <Route path="*" element={<Navigate to="/profile" replace />} />
              </Route>
            </Route>
            <Route path="*" element={<Navigate to="/profile" replace />} />
          </Routes>
        </AuthProvider>
      </HashRouter>
    </ThemeProvider>
  </React.StrictMode>
);
