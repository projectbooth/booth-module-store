/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Standalone dev-harness config today (ADR 0027: this repo builds against a mocked
// shell until booth-design's real shell/component library exists — see
// src/devshell/DevShell.tsx). The production build target is a library entry
// (ModuleStoreApp) that booth-design's shell will eventually import; the dev server
// here just wraps it in enough chrome to develop against locally.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Local dev only: booth-core's gateway would normally proxy
      // /modules/module-store/* to this service's own /api/* routes in a real
      // deployment. Pointed at BOOTH_MODULE_STORE_DEV_BACKEND when set, so `npm run
      // dev` can hit a locally running Go backend without CORS juggling.
      "/api": {
        target: process.env.BOOTH_MODULE_STORE_DEV_BACKEND ?? "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/setupTests.ts"],
  },
});
