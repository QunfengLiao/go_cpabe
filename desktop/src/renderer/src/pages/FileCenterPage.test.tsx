import { describe, expect, it } from "vitest";
import { FILE_CENTER_SCOPE_TABS } from "./FileCenterPage";

describe("文件中心页面契约", () => {
  it("只展示全部密文和我的加密文件，解密能力不进入列表状态", () => {
    expect(FILE_CENTER_SCOPE_TABS.map((tab) => tab.key)).toEqual(["tenant_cloud", "owned_by_me"]);
    expect(FILE_CENTER_SCOPE_TABS.map((tab) => tab.label)).toEqual(["全部密文", "我的加密文件"]);
  });
});
