import { Navigate } from "react-router-dom";

export function ReceivedFilesPage() {
  return <Navigate to="/file-center?tab=tenant_cloud" replace />;
}
