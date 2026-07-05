import { createContext, useContext, useEffect, useMemo, useState } from "react";
import type { EffectiveMode, ThemeMode, ThemeName } from "./themes";

const THEME_KEY = "go_cpabe_theme";
const MODE_KEY = "go_cpabe_theme_mode";
const DARK_QUERY = "(prefers-color-scheme: dark)";

interface ThemeContextValue {
  theme: ThemeName;
  mode: ThemeMode;
  effectiveMode: EffectiveMode;
  setTheme: (theme: ThemeName) => void;
  setMode: (mode: ThemeMode) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function storedTheme(): ThemeName {
  const value = localStorage.getItem(THEME_KEY);
  return isThemeName(value) ? value : "blue";
}

function storedMode(): ThemeMode {
  const value = localStorage.getItem(MODE_KEY);
  return value === "light" || value === "dark" || value === "system" ? value : "light";
}

function isThemeName(value: string | null): value is ThemeName {
  return value === "blue" || value === "purple" || value === "green" || value === "orange" || value === "slate";
}

function systemMode(): EffectiveMode {
  return window.matchMedia(DARK_QUERY).matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<ThemeName>(storedTheme);
  const [mode, setModeState] = useState<ThemeMode>(storedMode);
  const [systemPreference, setSystemPreference] = useState<EffectiveMode>(systemMode);
  const effectiveMode = mode === "system" ? systemPreference : mode;

  useEffect(() => {
    // System 模式需要监听 prefers-color-scheme，用户切换操作系统深色模式时页面才能即时同步。
    const media = window.matchMedia(DARK_QUERY);
    const onChange = () => setSystemPreference(media.matches ? "dark" : "light");
    onChange();
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  useEffect(() => {
    // 主题通过 html dataset 驱动 CSS Variables，避免把颜色硬编码散落在各个组件里。
    document.documentElement.dataset.theme = theme;
    document.documentElement.dataset.mode = effectiveMode;
    document.documentElement.style.colorScheme = effectiveMode;
  }, [theme, effectiveMode]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      mode,
      effectiveMode,
      setTheme(nextTheme) {
        // 外观设置独立于登录态，保存到 localStorage 后刷新仍能保留，同时不会污染 token/cachedAccounts。
        localStorage.setItem(THEME_KEY, nextTheme);
        setThemeState(nextTheme);
      },
      setMode(nextMode) {
        localStorage.setItem(MODE_KEY, nextMode);
        setModeState(nextMode);
      }
    }),
    [theme, mode, effectiveMode]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used inside ThemeProvider");
  }
  return ctx;
}
