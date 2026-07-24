import { beforeEach, describe, expect, it, vi } from "vitest";
import { confirmImport, getImportBatch, getImportBatchStatus, listImportBatches, validateImport } from "./import";
import { request } from "./request";

vi.mock("./request", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./request")>();
  return { ...actual, request: vi.fn() };
});

const requestMock = vi.mocked(request);

describe("租户批量导入 API", () => {
  beforeEach(() => requestMock.mockReset());

  it("用户和组织预校验使用当前租户路径且以 FormData 上传", async () => {
    requestMock.mockResolvedValue({ batch_id: "batch-1" });
    const file = new File(["xlsx"], "users.xlsx", { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });

    await validateImport("users", file);

    expect(requestMock).toHaveBeenCalledWith("/tenant/import/users/validate", expect.objectContaining({ method: "POST", body: expect.any(FormData) }));
  });

  it("确认、列表和详情接口不接受客户端 tenant_id", async () => {
    requestMock.mockResolvedValueOnce({ batch_id: "batch-1" }).mockResolvedValueOnce({ items: [] }).mockResolvedValueOnce({ batch_id: "batch-1" }).mockResolvedValueOnce({ batch_id: "batch-1", status: "QUEUED" });

    await confirmImport("org_units", "batch-1");
    await listImportBatches();
    await getImportBatch("batch-1");
    await getImportBatchStatus("batch-1");

    expect(requestMock).toHaveBeenNthCalledWith(1, "/tenant/import/org-units/confirm", { method: "POST", body: JSON.stringify({ batch_id: "batch-1" }) });
    expect(requestMock).toHaveBeenNthCalledWith(2, "/tenant/import/batches");
    expect(requestMock).toHaveBeenNthCalledWith(3, "/tenant/import/batches/batch-1");
    expect(requestMock).toHaveBeenNthCalledWith(4, "/tenant/import/batches/batch-1/status");
  });
});
