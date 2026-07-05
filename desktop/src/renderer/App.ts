import { getApiBaseUrl, getAppEnv, getAppVersion } from "./api/config";
import { getHealth, type DependencyStatus, type HealthResponse } from "./api/healthApi";

type NavItem = "总览" | "文件库" | "上传文件" | "访问策略" | "用户属性" | "密钥管理" | "系统状态";
type HealthUiState = "waiting" | "loading" | "online" | "degraded" | "offline";

interface RecentFile {
  name: string;
  size: string;
  algorithm: string;
  policy: string;
  status: string;
  updatedAt: string;
}

interface AppState {
  activeNav: NavItem;
  selectedFile?: RecentFile;
  healthState: HealthUiState;
  health?: HealthResponse;
  healthError?: string;
}

const navItems: NavItem[] = ["总览", "文件库", "上传文件", "访问策略", "用户属性", "密钥管理", "系统状态"];

const recentFiles: RecentFile[] = [
  {
    name: "crypto-lab-report.pdf",
    size: "2.8 MB",
    algorithm: "CP-ABE",
    policy: "role:teacher AND lab:crypto",
    status: "已加密",
    updatedAt: "2026-07-05 10:24"
  },
  {
    name: "project-a-dataset.zip",
    size: "148 MB",
    algorithm: "CP-ABE",
    policy: "project:A AND expire:valid",
    status: "可共享",
    updatedAt: "2026-07-04 18:36"
  },
  {
    name: "graduate-users.xlsx",
    size: "860 KB",
    algorithm: "RSA-OAEP",
    policy: "role:student OR role:admin",
    status: "策略已绑定",
    updatedAt: "2026-07-04 09:12"
  },
  {
    name: "health-audit.json",
    size: "64 KB",
    algorithm: "AES-GCM",
    policy: "role:admin",
    status: "本地暂存",
    updatedAt: "2026-07-03 21:08"
  }
];

const policyTemplates = ["role:teacher AND lab:crypto", "project:A AND expire:valid", "role:student OR role:admin"];

const appState: AppState = {
  activeNav: "总览",
  healthState: "waiting"
};

let appRoot: HTMLElement | undefined;

export function createApp(): HTMLElement {
  appRoot = document.createElement("main");
  appRoot.className = "app-shell";

  renderApp();
  void refreshHealthStatus();
  return appRoot;
}

function renderApp(): void {
  if (!appRoot) {
    return;
  }

  appRoot.replaceChildren(createSidebar(), createWorkspace());
}

function createSidebar(): HTMLElement {
  const sidebar = document.createElement("aside");
  sidebar.className = "sidebar";

  const brand = document.createElement("div");
  brand.className = "sidebar-brand";

  const mark = document.createElement("div");
  mark.className = "brand-mark";
  mark.textContent = "CP";

  const brandText = document.createElement("div");
  brandText.append(createText("strong", "CP-ABE"), createText("span", "加密文件共享"));
  brand.append(mark, brandText);

  const nav = document.createElement("nav");
  nav.className = "sidebar-nav";
  nav.setAttribute("aria-label", "主导航");

  for (const item of navItems) {
    const button = document.createElement("button");
    button.className = item === appState.activeNav ? "nav-item nav-item-active" : "nav-item";
    button.type = "button";
    button.textContent = item;
    button.addEventListener("click", () => {
      appState.activeNav = item;
      renderApp();
      if (item === "系统状态" && appState.healthState === "waiting") {
        void refreshHealthStatus();
      }
    });
    nav.append(button);
  }

  const footer = document.createElement("div");
  footer.className = "sidebar-footer";
  footer.append(
    createSidebarMeta("当前模式", "本地开发"),
    createSidebarMeta("当前用户", "管理员占位"),
    createSidebarMeta("版本", getAppVersion())
  );

  sidebar.append(brand, nav, footer);
  return sidebar;
}

function createWorkspace(): HTMLElement {
  const workspace = document.createElement("section");
  workspace.className = "workspace";
  workspace.append(createTopbar(), createCurrentPage());
  return workspace;
}

function createTopbar(): HTMLElement {
  const topbar = document.createElement("header");
  topbar.className = "topbar";

  const titleGroup = document.createElement("div");
  titleGroup.className = "topbar-title";
  titleGroup.append(createText("h1", "CP-ABE 加密文件共享系统"), createText("p", "文件保险库工作台"));

  const search = document.createElement("input");
  search.className = "search-input";
  search.type = "search";
  search.placeholder = "搜索文件、策略或用户属性";
  search.setAttribute("aria-label", "搜索文件、策略或用户属性");

  const statusStrip = document.createElement("div");
  statusStrip.className = "topbar-status";
  statusStrip.append(
    createInlineStatus("后端", getBackendStatusText(), getTopbarBadgeClass(appState.healthState)),
    createInlineStatus("MySQL", getDependencyStatusText("mysql"), getDependencyBadgeClass("mysql")),
    createInlineStatus("Redis", getDependencyStatusText("redis"), getDependencyBadgeClass("redis")),
    createInlineStatus("环境", getAppEnv(), "badge-env")
  );

  topbar.append(titleGroup, search, statusStrip);
  return topbar;
}

function createCurrentPage(): HTMLElement {
  if (appState.activeNav === "系统状态") {
    return createSystemStatusPage();
  }

  return createDashboard();
}

function createDashboard(): HTMLElement {
  const dashboard = document.createElement("div");
  dashboard.className = "dashboard";

  const content = document.createElement("section");
  content.className = "dashboard-main";
  content.append(createDashboardStats(), createUploadPanel(), createRecentFilesTable(), createPolicyTemplates());

  dashboard.append(content, createDetailPanel());
  return dashboard;
}

function createDashboardStats(): HTMLElement {
  const stats = [
    { label: "文件总数", value: "128", trend: "文件库占位数据" },
    { label: "已加密文件", value: "96", trend: "AES-GCM 内容加密" },
    { label: "可访问文件", value: "42", trend: "属性满足策略" },
    { label: "策略数量", value: "12", trend: "CP-ABE 表达式模板" },
    { label: "密钥状态", value: "就绪", trend: "DEK 封装占位" }
  ];

  const section = document.createElement("section");
  section.className = "stats-grid stats-grid-five";
  section.setAttribute("aria-label", "工作台统计");

  for (const stat of stats) {
    const card = document.createElement("article");
    card.className = "stat-card";
    card.append(createText("span", stat.label), createText("strong", stat.value), createText("small", stat.trend));
    section.append(card);
  }

  return section;
}

function createUploadPanel(): HTMLElement {
  const panel = createPanel("快速上传");
  panel.classList.add("upload-panel");

  const dropzone = document.createElement("button");
  dropzone.className = "upload-dropzone";
  dropzone.type = "button";
  dropzone.append(
    createText("strong", "拖拽文件到此处，或点击选择文件"),
    createText("span", "占位入口：后续接入 AES-GCM 文件加密、DEK 生成与 CP-ABE 封装")
  );

  panel.append(dropzone);
  return panel;
}

function createRecentFilesTable(): HTMLElement {
  const panel = createPanel("最近文件");
  panel.classList.add("table-panel");

  const tableContainer = document.createElement("div");
  tableContainer.className = "table-container";

  const table = document.createElement("table");
  table.className = "file-table";

  const thead = document.createElement("thead");
  const headRow = document.createElement("tr");
  for (const heading of ["文件名", "大小", "加密方式", "访问策略", "状态", "更新时间", "操作"]) {
    headRow.append(createText("th", heading));
  }
  thead.append(headRow);

  const tbody = document.createElement("tbody");
  for (const file of recentFiles) {
    const row = document.createElement("tr");
    if (appState.selectedFile?.name === file.name) {
      row.className = "selected-row";
    }
    row.addEventListener("click", () => {
      appState.selectedFile = file;
      renderApp();
    });
    row.append(
      createCell(file.name, "file-name"),
      createCell(file.size),
      createCell(file.algorithm),
      createCell(file.policy, "policy-expression"),
      createStatusCell(file.status),
      createCell(file.updatedAt),
      createActionCell(file)
    );
    tbody.append(row);
  }

  table.append(thead, tbody);
  tableContainer.append(table);
  panel.append(tableContainer);
  return panel;
}

function createPolicyTemplates(): HTMLElement {
  const panel = createPanel("访问策略模板");
  panel.classList.add("policy-panel");

  const list = document.createElement("div");
  list.className = "policy-list";

  for (const template of policyTemplates) {
    const item = document.createElement("button");
    item.className = "policy-chip";
    item.type = "button";
    item.textContent = template;
    list.append(item);
  }

  panel.append(list);
  return panel;
}

function createDetailPanel(): HTMLElement {
  const file = appState.selectedFile;
  const panel = document.createElement("aside");
  panel.className = "detail-panel";
  panel.append(createText("h2", "加密详情"));

  const rows = [
    ["当前选中文件", file?.name || "暂无选择"],
    ["加密方式", file?.algorithm || "CP-ABE"],
    ["对称加密", "AES-GCM"],
    ["DEK 封装", file?.algorithm === "RSA-OAEP" ? "RSA-OAEP" : "CP-ABE"],
    ["策略校验", file ? "等待检测" : "未选择"],
    ["后端地址", getApiBaseUrl()]
  ];

  for (const [label, value] of rows) {
    panel.append(createDetailRow(label, value));
  }

  return panel;
}

function createSystemStatusPage(): HTMLElement {
  const page = document.createElement("section");
  page.className = "system-status-page";

  const header = document.createElement("div");
  header.className = "page-header";
  const titleGroup = document.createElement("div");
  titleGroup.append(createText("h2", "系统状态"), createText("p", "后端服务、数据库和缓存依赖的运行状态"));

  const refreshButton = document.createElement("button");
  refreshButton.className = "primary-action";
  refreshButton.type = "button";
  refreshButton.textContent = appState.healthState === "loading" ? "检测中..." : "重新检测";
  refreshButton.disabled = appState.healthState === "loading";
  refreshButton.addEventListener("click", () => {
    void refreshHealthStatus();
  });

  header.append(titleGroup, refreshButton);

  const summary = document.createElement("section");
  summary.className = "status-summary panel";
  summary.append(
    createText("span", "总体状态"),
    createStatusBadge(getOverallStatusText(), getTopbarBadgeClass(appState.healthState)),
    createText("p", getOverallStatusDescription())
  );

  const grid = document.createElement("section");
  grid.className = "health-grid";
  grid.append(
    createHealthStatusCard("后端服务", getBackendStatusText(), getTopbarBadgeClass(appState.healthState), [
      ["后端地址", getApiBaseUrl()],
      ["当前环境", appState.health?.app.env || getAppEnv()],
      ["最近检查时间", getCheckedAtText()]
    ]),
    createHealthStatusCard("MySQL", getDependencyStatusText("mysql"), getDependencyBadgeClass("mysql"), [
      ["状态说明", appState.health?.mysql.message || getWaitingText()],
      ["错误信息", getDependencyErrorText("mysql")]
    ]),
    createHealthStatusCard("Redis", getDependencyStatusText("redis"), getDependencyBadgeClass("redis"), [
      ["状态说明", appState.health?.redis.message || getWaitingText()],
      ["错误信息", getDependencyErrorText("redis")]
    ]),
    createHealthStatusCard("配置信息", "配置", "badge-muted", [
      ["API 地址", getApiBaseUrl()],
      ["当前环境", getAppEnv()],
      ["应用版本", getAppVersion()]
    ])
  );

  if (appState.healthError) {
    const error = document.createElement("div");
    error.className = "health-error";
    error.textContent = appState.healthError;
    page.append(header, summary, error, grid);
    return page;
  }

  page.append(header, summary, grid);
  return page;
}

function createHealthStatusCard(
  title: string,
  statusText: string,
  badgeClass: string,
  rows: Array<[string, string]>
): HTMLElement {
  const card = document.createElement("article");
  card.className = "health-card";

  const header = document.createElement("div");
  header.className = "health-card-header";
  header.append(createText("h3", title), createStatusBadge(statusText, badgeClass));
  card.append(header);

  for (const [label, value] of rows) {
    card.append(createDetailRow(label, value));
  }

  return card;
}

async function refreshHealthStatus(): Promise<void> {
  appState.healthState = "loading";
  appState.healthError = undefined;
  renderApp();

  try {
    const health = await getHealth();
    appState.health = health;
    appState.healthState = health.status === "ok" ? "online" : "degraded";
  } catch {
    appState.health = undefined;
    appState.healthState = "offline";
    appState.healthError = "无法连接后端服务，请确认 Go 后端是否已启动，或检查 DESKTOP_API_BASE_URL 配置。";
  }

  renderApp();
}

function createPanel(titleText: string): HTMLElement {
  const panel = document.createElement("section");
  panel.className = "panel";

  const header = document.createElement("div");
  header.className = "panel-header";
  header.append(createText("h2", titleText));
  panel.append(header);

  return panel;
}

function createSidebarMeta(label: string, value: string): HTMLElement {
  const row = document.createElement("div");
  row.className = "sidebar-meta";
  row.append(createText("span", label), createText("strong", value));
  return row;
}

function createInlineStatus(label: string, value: string, badgeClass: string): HTMLElement {
  const item = document.createElement("div");
  item.className = "inline-status";
  item.append(createText("span", `${label}：`), createStatusBadge(value, badgeClass));
  return item;
}

function createStatusBadge(text: string, className: string): HTMLElement {
  const badge = document.createElement("span");
  badge.className = `badge ${className}`;
  badge.textContent = text;
  return badge;
}

function createCell(text: string, className?: string): HTMLTableCellElement {
  const td = document.createElement("td");
  if (className) {
    td.className = className;
  }
  td.textContent = text;
  return td;
}

function createStatusCell(text: string): HTMLTableCellElement {
  const td = document.createElement("td");
  td.append(createStatusBadge(text, "badge-status"));
  return td;
}

function createActionCell(file: RecentFile): HTMLTableCellElement {
  const td = document.createElement("td");
  const group = document.createElement("div");
  group.className = "table-actions";

  for (const label of ["查看", "策略", "下载"]) {
    const button = document.createElement("button");
    button.className = "table-action";
    button.type = "button";
    button.textContent = label;
    button.addEventListener("click", (event) => {
      event.stopPropagation();
      appState.selectedFile = file;
      renderApp();
    });
    group.append(button);
  }

  td.append(group);
  return td;
}

function createDetailRow(label: string, value: string): HTMLElement {
  const row = document.createElement("div");
  row.className = "detail-row";
  row.append(createText("span", label), createText("strong", value));
  return row;
}

function createText<K extends keyof HTMLElementTagNameMap>(tag: K, text: string): HTMLElementTagNameMap[K] {
  const element = document.createElement(tag);
  element.textContent = text;
  return element;
}

function getBackendStatusText(): string {
  switch (appState.healthState) {
    case "loading":
      return "检测中";
    case "online":
      return "正常";
    case "degraded":
      return "部分异常";
    case "offline":
      return "离线";
    default:
      return "等待检测";
  }
}

function getOverallStatusText(): string {
  if (appState.healthState === "online") {
    return "正常";
  }
  if (appState.healthState === "degraded") {
    return "部分异常";
  }
  if (appState.healthState === "offline") {
    return "离线";
  }
  if (appState.healthState === "loading") {
    return "检测中";
  }
  return "等待检测";
}

function getOverallStatusDescription(): string {
  if (appState.healthState === "loading") {
    return "正在请求后端 GET /health 接口。";
  }
  if (appState.healthState === "offline") {
    return appState.healthError || "无法连接后端服务。";
  }
  if (appState.healthState === "degraded") {
    return "后端服务可访问，但 MySQL 或 Redis 至少一个依赖异常。";
  }
  if (appState.healthState === "online") {
    return "后端服务、MySQL 和 Redis 均处于正常状态。";
  }
  return "尚未执行健康检查。";
}

function getTopbarBadgeClass(state: HealthUiState): string {
  if (state === "online") {
    return "badge-success";
  }
  if (state === "degraded" || state === "offline") {
    return "badge-error";
  }
  if (state === "loading") {
    return "badge-info";
  }
  return "badge-muted";
}

function getDependencyStatusText(kind: "mysql" | "redis"): string {
  if (appState.healthState === "loading") {
    return "检测中";
  }
  if (appState.healthState === "offline") {
    return "离线";
  }

  const status = appState.health?.[kind].status;
  if (!status) {
    return "等待检测";
  }
  return status === "ok" ? "正常" : "异常";
}

function getDependencyBadgeClass(kind: "mysql" | "redis"): string {
  if (appState.healthState === "loading") {
    return "badge-info";
  }
  if (appState.healthState === "offline") {
    return "badge-error";
  }

  const status = appState.health?.[kind].status;
  if (!status) {
    return "badge-muted";
  }
  return status === "ok" ? "badge-success" : "badge-error";
}

function getDependencyErrorText(kind: "mysql" | "redis"): string {
  if (appState.healthState === "offline") {
    return appState.healthError || "无法连接后端服务。";
  }

  const dependency = appState.health?.[kind] as { status: DependencyStatus; message: string } | undefined;
  if (!dependency) {
    return "暂无";
  }

  return dependency.status === "ok" ? "无" : dependency.message;
}

function getWaitingText(): string {
  return appState.healthState === "loading" ? "正在检测" : "等待检测";
}

function getCheckedAtText(): string {
  if (appState.healthState === "loading") {
    return "检测中";
  }
  if (!appState.health?.checkedAt) {
    return "暂无";
  }

  return new Date(appState.health.checkedAt).toLocaleString("zh-CN");
}
