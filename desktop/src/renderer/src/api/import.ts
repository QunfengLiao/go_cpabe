import { request, requestBinary } from "./request";

export type ImportType = "users" | "org_units";
export type ImportStatus = "UPLOADED" | "VALIDATING" | "VALIDATED" | "QUEUED" | "IMPORTING" | "SUCCEEDED" | "FAILED" | "EXPIRED";
export type ImportAction = "CREATE" | "UPDATE" | "SKIP";

export interface ImportError {
  row_number: number;
  field: string;
  code: string;
  message: string;
}

export interface ImportRow {
  row_number: number;
  key: string;
  action: ImportAction;
  status: "VALID" | "INVALID";
  fields: Record<string, string>;
  errors?: ImportError[];
}

export interface ImportPreview {
  batch_id: string;
  status: ImportStatus;
  limits: { max_file_size: number; max_rows: number; batch_ttl_seconds: number };
  summary: { total: number; valid: number; created: number; updated: number; skipped: number; failed: number };
  rows: ImportRow[];
}

export interface ImportBatch {
  batch_id: string;
  import_type: ImportType;
  file_name: string;
  total_count: number;
  valid_count: number;
  success_count: number;
  failure_count: number;
  skipped_count: number;
  status: ImportStatus;
  created_by: number;
  created_at: string;
}

export interface ImportBatchStatusView {
  batch_id: string;
  status: ImportStatus;
  phase: string;
  total: number;
  processed: number;
  success: number;
  failed: number;
  skipped: number;
  attempt_count: number;
  failure_reason?: string;
}

export function templatePath(type: ImportType): string {
  return type === "users" ? "/tenant/import/templates/users" : "/tenant/import/templates/org-units";
}

export function validatePath(type: ImportType): string {
  return type === "users" ? "/tenant/import/users/validate" : "/tenant/import/org-units/validate";
}

export function confirmPath(type: ImportType): string {
  return type === "users" ? "/tenant/import/users/confirm" : "/tenant/import/org-units/confirm";
}

export async function downloadImportTemplate(type: ImportType): Promise<Blob> {
  return requestBinary(templatePath(type));
}

export async function validateImport(type: ImportType, file: File): Promise<ImportPreview> {
  const form = new FormData();
  form.append("file", file);
  return request<ImportPreview>(validatePath(type), { method: "POST", body: form });
}

export async function confirmImport(type: ImportType, batchId: string): Promise<ImportPreview> {
  return request<ImportPreview>(confirmPath(type), { method: "POST", body: JSON.stringify({ batch_id: batchId }) });
}

export async function listImportBatches(): Promise<ImportBatch[]> {
  const data = await request<{ items: ImportBatch[] }>("/tenant/import/batches");
  return data.items;
}

export async function getImportBatch(batchId: string): Promise<ImportPreview> {
  return request<ImportPreview>(`/tenant/import/batches/${encodeURIComponent(batchId)}`);
}

export async function getImportBatchStatus(batchId: string): Promise<ImportBatchStatusView> {
  return request<ImportBatchStatusView>(`/tenant/import/batches/${encodeURIComponent(batchId)}/status`);
}

export async function downloadImportErrors(batchId: string): Promise<Blob> {
  return requestBinary(`/tenant/import/batches/${encodeURIComponent(batchId)}/errors`);
}
