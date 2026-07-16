# Electron 桌面端

桌面端负责选择本地文件、调用固定的 Go Crypto Worker、保存本机 RSA 私钥、上传密文并展示
真实进度。远程后端不接收文件明文、明文数据密钥 DEK、私钥或本地路径。

## 本地加密与密钥

- 原生文件选择器只向渲染层返回一次性文件句柄和展示元数据，不返回完整路径。
- Go Crypto Worker 使用 4 MiB 认证分块 AES-256-GCM 加密内容，并用首个真实
  `RSA-OAEP-SHA256` 适配器保护 DEK。多接收者加密仍只生成一份文件密文和一个 DEK，
  Worker 再使用每位接收者选定的 RSA 公钥分别生成一份 protected DEK，避免重复加密文件内容。
- Electron 主进程使用同步 `safeStorage.encryptString` 保护 PKCS#8 私钥。Linux 的
  `basic_text` 或 `unknown` 后端会被拒绝，持久文件中不得出现明文 PEM 私钥头。
- RSA 是首个实现方案；协调器、容器、上传、任务、补偿和审计不依赖 RSA。下一期优先评估
  真实 Go TKN20，不展示伪 CP-ABE。

## 文件中心与本地解密

- 文件中心统一提供“企业云盘”“分享给我”“我的加密文件”三个 scope。能看到文件的用户都可以
  下载原始密文包和密钥信封；RBAC 只控制文件可见性与管理动作，不控制最终解密结果。
- 本地解密由 Electron 主进程完成：用户先选择输出目录，主进程下载并校验密文 SHA-256，
  按密钥信封的 `key_id` 或公钥指纹在本地安全存储中匹配私钥，再调用 Go Crypto Worker 解封 DEK 和还原文件。
- 同名明文不会被覆盖，而是自动生成带编号的新文件名。渲染层只获得文件名和 reveal token，
  不会获得完整本地路径、RSA 私钥、明文 DEK 或 protected DEK。
- 解密成功后只使用 `shell.showItemInFolder` 定位文件，不自动运行或直接打开明文文件。

## 失败恢复与清理

失败、取消和退出路径都会尽力终止 Worker、取消上传、释放文件句柄并删除明文 `.part`；解密失败或
找不到匹配私钥时会保留已下载的原始密文包。启动时只
巡检过期临时密文，不保存或恢复 DEK，也不自动继续读取旧文件；重试需要创建新执行并重新
加密。

本地解密先写入独立临时目录中的 `.part-*` 明文文件，只有 Worker 完整成功后才重命名到用户
选择的目录。RSA 解封、AES-GCM 完整性校验、写盘或重命名任一步失败时，主进程都会在
`finally` 中删除 `.part-*` 和临时目录，并且不会调用打开文件夹，避免残留可被误用的部分明文；
原始密文包已移出临时目录时不再删除。

## 开发验证

```powershell
npm.cmd run typecheck
npm.cmd test
npm.cmd run build
```

开发默认 API 为 `http://localhost:18080/api/v1`。生产打包必须携带固定 Go Worker，并设置
`CRYPTO_WORKER_SHA256` 以校验完整性；主进程以 `shell: false` 启动单用途子进程，渲染层保持
`contextIsolation=true` 和 `nodeIntegration=false`。
