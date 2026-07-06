export type ThemeName = "blue" | "purple" | "green" | "orange" | "slate";
export type ThemeMode = "light" | "dark" | "system";
export type EffectiveMode = "light" | "dark";

export interface ThemeOption {
  value: ThemeName;
  label: string;
  description: string;
  swatch: string;
}

export interface ModeOption {
  value: ThemeMode;
  label: string;
  description: string;
}

export const themeOptions: ThemeOption[] = [
  { value: "blue", label: "Blue", description: "科技、安全", swatch: "#1c5d99" },
  { value: "purple", label: "Purple", description: "AI、未来感", swatch: "#7c3aed" },
  { value: "green", label: "Green", description: "安全、可信", swatch: "#12805c" },
  { value: "orange", label: "Orange", description: "活跃、提示", swatch: "#c76a1d" },
  { value: "slate", label: "Slate", description: "企业后台", swatch: "#475569" }
];

export const modeOptions: ModeOption[] = [
  { value: "light", label: "浅色", description: "明亮清爽" },
  { value: "dark", label: "深色", description: "低亮护眼" },
  { value: "system", label: "跟随系统", description: "自动适配" }
];
