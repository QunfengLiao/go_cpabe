import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import { modeOptions, themeOptions } from "../theme/themes";
import { useTheme } from "../theme/ThemeProvider";

function PaletteIcon() {
  return (
    <svg className="theme-trigger-icon" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M12 3a9 9 0 0 0 0 18h1.5a1.8 1.8 0 0 0 1.2-3.15 1.8 1.8 0 0 1 1.2-3.15H18a3 3 0 0 0 3-3C21 6.9 16.95 3 12 3Z" />
      <circle cx="7.5" cy="10" r="1" />
      <circle cx="10" cy="6.8" r="1" />
      <circle cx="14.3" cy="7.2" r="1" />
      <circle cx="16.5" cy="10.7" r="1" />
    </svg>
  );
}

export function ThemeSwitcher({ compact = false }: { compact?: boolean }) {
  const { theme, mode, setTheme, setMode } = useTheme();
  const [open, setOpen] = useState(false);
  const [panelStyle, setPanelStyle] = useState<CSSProperties>({});
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const panelRef = useRef<HTMLElement | null>(null);

  const updatePanelPosition = useCallback(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    const rect = trigger.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const margin = 12;
    const gap = 10;
    const panelWidth = Math.min(360, viewportWidth - margin * 2);
    const estimatedPanelHeight = Math.min(430, viewportHeight - margin * 2);
    const openToRight = compact && rect.right + gap + panelWidth <= viewportWidth - margin;
    const rawLeft = openToRight ? rect.right + gap : rect.right - panelWidth;
    const left = Math.min(Math.max(margin, rawLeft), viewportWidth - panelWidth - margin);
    const bottomTop = rect.bottom + gap;
    const rawTop = bottomTop + estimatedPanelHeight <= viewportHeight - margin ? bottomTop : rect.top - estimatedPanelHeight - gap;
    const top = Math.min(Math.max(margin, rawTop), viewportHeight - estimatedPanelHeight - margin);
    setPanelStyle({ left, top, width: panelWidth });
  }, [compact]);

  useLayoutEffect(() => {
    if (!open) return;
    updatePanelPosition();
  }, [open, updatePanelPosition]);

  useEffect(() => {
    if (!open) return;

    function onPointerDown(event: PointerEvent) {
      const target = event.target as Node;
      if (triggerRef.current?.contains(target) || panelRef.current?.contains(target)) return;
      setOpen(false);
    }

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("resize", updatePanelPosition);
    window.addEventListener("scroll", updatePanelPosition, true);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("resize", updatePanelPosition);
      window.removeEventListener("scroll", updatePanelPosition, true);
    };
  }, [open, updatePanelPosition]);

  return (
    <div className={compact ? "theme-switcher theme-switcher-compact" : "theme-switcher"}>
      <button
        ref={triggerRef}
        className="theme-trigger"
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-label="外观设置"
        title="外观设置"
      >
        <PaletteIcon />
      </button>
      {open &&
        createPortal(
          <section className="theme-panel" aria-label="外观设置" ref={panelRef} style={panelStyle}>
            <div className="theme-panel-header">
              <strong>外观设置</strong>
              <span>主题色与显示模式</span>
            </div>
            <div className="theme-section">
              <span className="theme-section-title">主题色</span>
              <div className="theme-option-grid">
                {themeOptions.map((option) => (
                  <button
                    className={theme === option.value ? "theme-option theme-option-active" : "theme-option"}
                    key={option.value}
                    onClick={() => setTheme(option.value)}
                    type="button"
                  >
                    <span className="theme-swatch" style={{ background: option.swatch }} />
                    <span>
                      <strong>{option.label}</strong>
                      <small>{option.description}</small>
                    </span>
                  </button>
                ))}
              </div>
            </div>
            <div className="theme-section">
              <span className="theme-section-title">显示模式</span>
              <div className="mode-option-grid">
                {modeOptions.map((option) => (
                  <button className={mode === option.value ? "mode-option mode-option-active" : "mode-option"} key={option.value} onClick={() => setMode(option.value)} type="button">
                    <strong>{option.label}</strong>
                    <small>{option.description}</small>
                  </button>
                ))}
              </div>
            </div>
          </section>,
          document.body
        )}
    </div>
  );
}
