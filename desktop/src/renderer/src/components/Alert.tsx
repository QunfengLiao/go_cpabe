interface AlertProps {
  type?: "error" | "success" | "info";
  message: string;
}

export function Alert({ type = "info", message }: AlertProps) {
  if (!message) return null;
  return <div className={`alert alert-${type}`}>{message}</div>;
}
