import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "./",
  plugins: [react()],
  root: "src/renderer",
  publicDir: "../../public",
  envDir: ".",
  build: {
    outDir: "../../dist/renderer",
    emptyOutDir: true
  }
});
