import { contextBridge } from "electron";

contextBridge.exposeInMainWorld("desktopRuntime", {
  platform: process.platform
});
