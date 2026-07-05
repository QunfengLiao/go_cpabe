import fs from "node:fs";
import path from "node:path";
import { defineConfig, loadEnv } from "vite";

const packageJson = JSON.parse(fs.readFileSync(path.resolve(__dirname, "package.json"), "utf-8")) as {
  version?: string;
};

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, "..", "");

  return {
    root: ".",
    base: "./",
    envDir: "..",
    envPrefix: ["VITE_", "DESKTOP_"],
    define: {
      __APP_ENV__: JSON.stringify(env.APP_ENV || "development"),
      __APP_VERSION__: JSON.stringify(packageJson.version || "0.1.0")
    },
    build: {
      outDir: "dist/renderer",
      emptyOutDir: false,
      rollupOptions: {
        input: "index.html"
      }
    }
  };
});
