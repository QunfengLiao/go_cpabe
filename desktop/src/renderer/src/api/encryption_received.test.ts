import { beforeEach, describe, expect, it, vi } from "vitest";
import { listReceivedFiles } from "./encryption";
import { request } from "./request";

vi.mock("./request", () => ({ API_BASE_URL: "http://localhost:18080/api/v1", ApiError: class extends Error {}, clearRequestCache: vi.fn(), request: vi.fn() }));
vi.mock("./authRuntime", () => ({ getAuthRuntime: vi.fn() }));

describe("收到的文件 API", () => {
  beforeEach(() => vi.mocked(request).mockReset());

  it("使用当前接收者范围接口而不是所有者文件接口", async () => {
    vi.mocked(request).mockResolvedValue({ items: [], total: 0, page: 2, page_size: 10 });
    await listReceivedFiles(2, 10);
    expect(request).toHaveBeenCalledWith("/tenant/received-files?page=2&page_size=10");
  });
});
