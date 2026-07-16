import { describe, expect, it } from "vitest";
import { decryptActionText, fileTableColumnTitles } from "./FileTable";

describe("文件中心表格列", () => {
  it("所有文件列表只展示密文仓库字段", () => {
    const expected = ["文件名", "文件拥有者", "加密算法", "密文大小", "创建时间", "下载操作"];
    expect(fileTableColumnTitles("tenant_cloud")).toEqual(expected);
    expect(fileTableColumnTitles("owned_by_me")).toEqual(expected);
  });

  it("主操作与本地文件状态一致", () => {
    expect(decryptActionText({ local_decryption_reveal_token: "token" })).toBe("打开");
    expect(decryptActionText({})).toBe("下载");
  });
});
