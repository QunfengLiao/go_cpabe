import type { TenantBranding, TenantSummary } from "../types";

const BRANDING_CACHE_PREFIX = "go_cpabe_tenant_branding:";

export const defaultTenantBranding: TenantBranding = {
  primaryColor: "#1c5d99",
  sidebarColor: "#174f86",
  backgroundStart: "#eef3f8",
  backgroundEnd: "#f8fbff",
  backgroundGlow: "#7db7e8"
};

export const platformBranding: TenantBranding = {
  ...defaultTenantBranding,
  primaryColor: "#475569",
  sidebarColor: "#334155",
  backgroundStart: "#f4f7fb",
  backgroundEnd: "#eef3f8",
  backgroundGlow: "#94a3b8"
};

export function demoTenantBranding(code?: string | null): TenantBranding {
  switch (normalizeTenantCode(code)) {
    case "scnu":
      return {
        logoUrl: "/tenant-branding/scnu/logo.png",
        loginBackgroundUrl: "/tenant-branding/scnu/logo.png",
        workspaceBackgroundUrl: "/tenant-branding/scnu/logo.png",
        primaryColor: "#1c5d99",
        sidebarColor: "#1d4f91",
        backgroundStart: "#f7fbff",
        backgroundEnd: "#fffaf0",
        backgroundGlow: "#7db7e8"
      };
    case "sangfor":
      return {
        logoUrl: "/tenant-branding/sangfor/logo.png",
        loginBackgroundUrl: "/tenant-branding/sangfor/logo.png",
        workspaceBackgroundUrl: "/tenant-branding/sangfor/logo.png",
        primaryColor: "#183b73",
        sidebarColor: "#102a55",
        backgroundStart: "#f3f6fa",
        backgroundEnd: "#e8eef6",
        backgroundGlow: "#4f8edb"
      };
    case "aia":
    case "aia-hk":
      return {
        logoUrl: "/tenant-branding/aia/logo.png",
        loginBackgroundUrl: "/tenant-branding/aia/logo.png",
        workspaceBackgroundUrl: "/tenant-branding/aia/logo.png",
        primaryColor: "#d71920",
        sidebarColor: "#b5121b",
        backgroundStart: "#fffafa",
        backgroundEnd: "#f7f8fb",
        backgroundGlow: "#f05a61"
      };
    default:
      return defaultTenantBranding;
  }
}

export function normalizeTenantBranding(branding?: TenantBranding | null): TenantBranding {
  return {
    logoUrl: nonEmpty(branding?.logoUrl),
    loginBackgroundUrl: nonEmpty(branding?.loginBackgroundUrl),
    workspaceBackgroundUrl: nonEmpty(branding?.workspaceBackgroundUrl),
    primaryColor: nonEmpty(branding?.primaryColor),
    sidebarColor: nonEmpty(branding?.sidebarColor),
    backgroundStart: nonEmpty(branding?.backgroundStart),
    backgroundEnd: nonEmpty(branding?.backgroundEnd),
    backgroundGlow: nonEmpty(branding?.backgroundGlow)
  };
}

export function brandingForTenant(tenant?: TenantSummary | null, isPlatform = false): TenantBranding {
  if (!tenant && isPlatform) return platformBranding;
  return { ...defaultTenantBranding, ...demoTenantBranding(tenantCodeOf(tenant)), ...normalizeTenantBranding(tenant?.branding) };
}

export function cacheTenantBranding(tenant?: TenantSummary | null): void {
  if (!tenant?.tenant_id) return;
  localStorage.setItem(brandingCacheKey(tenant.tenant_id), JSON.stringify(brandingForTenant(tenant)));
}

export function cachedTenantBranding(tenantId?: number | string | null): TenantBranding | null {
  if (!tenantId) return null;
  const raw = localStorage.getItem(brandingCacheKey(tenantId));
  if (!raw) return null;
  try {
    return normalizeTenantBranding(JSON.parse(raw) as TenantBranding);
  } catch {
    localStorage.removeItem(brandingCacheKey(tenantId));
    return null;
  }
}

export function applyTenantBranding(tenant?: TenantSummary | null, options: { isPlatform?: boolean; clearing?: boolean } = {}): void {
  const root = document.documentElement;
  const branding = brandingForTenant(tenant, options.isPlatform);
  const logoUrl = options.clearing ? "" : branding.logoUrl ?? "";
  const loginBackgroundUrl = options.clearing ? "" : branding.loginBackgroundUrl ?? "";
  const workspaceBackgroundUrl = options.clearing ? "" : branding.workspaceBackgroundUrl ?? "";

  root.dataset.tenantCode = tenantCodeOf(tenant) ?? (options.isPlatform ? "platform" : "default");
  root.style.setProperty("--tenant-logo-url", cssURL(logoUrl));
  root.style.setProperty("--tenant-login-bg-url", cssURL(loginBackgroundUrl));
  root.style.setProperty("--tenant-workspace-bg-url", cssURL(workspaceBackgroundUrl));
  root.style.setProperty("--tenant-primary", branding.primaryColor ?? defaultTenantBranding.primaryColor!);
  root.style.setProperty("--tenant-sidebar", branding.sidebarColor ?? branding.primaryColor ?? defaultTenantBranding.sidebarColor!);
  root.style.setProperty("--tenant-bg-start", branding.backgroundStart ?? defaultTenantBranding.backgroundStart!);
  root.style.setProperty("--tenant-bg-end", branding.backgroundEnd ?? defaultTenantBranding.backgroundEnd!);
  root.style.setProperty("--tenant-bg-glow", branding.backgroundGlow ?? defaultTenantBranding.backgroundGlow!);
  root.style.setProperty("--color-primary", branding.primaryColor ?? defaultTenantBranding.primaryColor!);
  root.style.setProperty("--color-primary-hover", branding.sidebarColor ?? branding.primaryColor ?? defaultTenantBranding.sidebarColor!);
}

export async function preloadTenantBranding(tenant?: TenantSummary | null): Promise<void> {
  const branding = brandingForTenant(tenant);
  const urls = [branding.logoUrl, branding.loginBackgroundUrl, branding.workspaceBackgroundUrl].filter(Boolean) as string[];
  await Promise.all(urls.map(preloadImage));
}

export function mergeCachedBranding(tenant: TenantSummary): TenantSummary {
  const fallback = demoTenantBranding(tenantCodeOf(tenant));
  const cached = cachedTenantBranding(tenant.tenant_id);
  return { ...tenant, branding: { ...fallback, ...(cached ?? {}), ...normalizeTenantBranding(tenant.branding) } };
}

function brandingCacheKey(tenantId: number | string): string {
  return `${BRANDING_CACHE_PREFIX}${tenantId}`;
}

function cssURL(url: string): string {
  return url ? `url("${url.replace(/"/g, '\\"')}")` : "none";
}

function tenantCodeOf(tenant?: TenantSummary | null): string | undefined {
  return normalizeTenantCode(tenant?.tenant_code ?? tenant?.tenantCode);
}

function normalizeTenantCode(code?: string | null): string | undefined {
  const normalized = String(code ?? "").trim().toLowerCase();
  return normalized || undefined;
}

function nonEmpty(value?: string | null): string | undefined {
  const normalized = String(value ?? "").trim();
  return normalized || undefined;
}

function preloadImage(url: string): Promise<void> {
  return new Promise((resolve) => {
    const image = new Image();
    image.onload = () => resolve();
    image.onerror = () => resolve();
    image.src = url;
  });
}
