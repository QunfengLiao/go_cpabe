import { clearTenantStartupSession, saveLastTenantCode } from "./api/authStorage";
import { isKnownTenantCode } from "./config/tenantLoginConfigs";

const rawDefaultTenantCode = import.meta.env.VITE_DEFAULT_TENANT_CODE as string | undefined;
export const startupTenantCode = isKnownTenantCode(rawDefaultTenantCode) ? rawDefaultTenantCode : "";

export function loginPathForTenant(tenantCode: string): string {
  return `/login/${tenantCode}`;
}

export function prepareStartupTenant(): void {
  const explicitLoginTenant = tenantFromHash(window.location.hash);
  if (explicitLoginTenant) {
    clearTenantStartupSession();
    if (isKnownTenantCode(explicitLoginTenant)) {
      saveLastTenantCode(explicitLoginTenant);
    }
    return;
  }

  if (!startupTenantCode) return;

  // 开发租户命令代表本次启动意图，旧 token 和旧租户上下文不能覆盖它，否则会误进上一次账号。
  clearTenantStartupSession();
  saveLastTenantCode(startupTenantCode);
  if (window.location.hash !== `#${loginPathForTenant(startupTenantCode)}`) {
    window.location.hash = loginPathForTenant(startupTenantCode);
  }
}

function tenantFromHash(hash: string): string {
  const match = hash.match(/^#\/login\/([^/?#]+)/);
  return match ? decodeURIComponent(match[1]) : "";
}
