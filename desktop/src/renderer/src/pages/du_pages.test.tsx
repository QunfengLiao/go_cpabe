import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

describe("DU 独立菜单和本地解密边界", () => {
  it("密钥页和文件中心的权限边界独立", () => {
    const source = readFileSync(path.resolve(process.cwd(), "src/renderer/src/components/AppLayout.tsx"), "utf8");
    expect(source).toContain('auth.hasPermission("crypto.key.self.manage")');
    expect(source).toContain('key: "/my-rsa-keys"');
    expect(source).not.toContain('auth.hasPermission("file.read")');
    expect(source).toContain('key: "/file-center"');
    expect(source).toContain('label: "文件中心"');
  });

  it("文件中心只通过主进程 IPC 发起本地解密", () => {
    const source = readFileSync(path.resolve(process.cwd(), "src/renderer/src/pages/FileCenterPage.tsx"), "utf8");
    expect(source).toContain("desktopEncryption.decryptFile");
    expect(source).not.toContain("private_key_pem");
    expect(source).not.toContain("protected_key_base64");
  });
});
