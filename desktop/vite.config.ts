import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "./",
  plugins: [react()],
  root: "src/renderer",
  publicDir: "../../public",
  envDir: ".",
	test: {
	  include: ["src/**/*.test.{ts,tsx}", "../main/**/*.test.ts"],
	  environment: "node"
	},
  build: {
    outDir: "../../dist/renderer",
    emptyOutDir: true
  }
});
