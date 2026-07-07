import { useEffect, useState } from "react";

interface AlertProps {
  type?: "error" | "success" | "info";
  message: string;
  autoDismissMs?: number;
  onDismiss?: () => void;
}

const EXIT_ANIMATION_MS = 220;

export function Alert({ type = "info", message, autoDismissMs, onDismiss }: AlertProps) {
  const [leaving, setLeaving] = useState(false);

  useEffect(() => {
    setLeaving(false);
    if (!message || !autoDismissMs || !onDismiss) return;

    const leaveDelay = Math.max(0, autoDismissMs - EXIT_ANIMATION_MS);
    const leaveTimer = window.setTimeout(() => setLeaving(true), leaveDelay);
    const dismissTimer = window.setTimeout(onDismiss, autoDismissMs);

    return () => {
      window.clearTimeout(leaveTimer);
      window.clearTimeout(dismissTimer);
    };
  }, [autoDismissMs, message, onDismiss]);

  if (!message) return null;
  return <div className={`alert alert-${type}${leaving ? " alert-leaving" : ""}`}>{message}</div>;
}
