import React from "react";
import { createRoot } from "react-dom/client";
import { HashRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { GuestOnly } from "./auth/GuestOnly";
import { RequireAuth } from "./auth/RequireAuth";
import { AppLayout } from "./components/AppLayout";
import { GuestLayout } from "./components/GuestLayout";
import { LoginPage } from "./pages/LoginPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RegisterPage } from "./pages/RegisterPage";
import { SelectTenantPage } from "./pages/SelectTenantPage";
import { StartupRedirect } from "./pages/StartupRedirect";
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
