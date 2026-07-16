import { Navigate } from "react-router-dom";
import type { EncryptedFileRecord } from "../api/encryption";

export function canDownloadEncryptedFile(record: EncryptedFileRecord): boolean {
  return record.status === "AVAILABLE";
}

export function EncryptedFilesPage() {
  return <Navigate to="/file-center?tab=owned_by_me" replace />;
}
