/// <reference types="vitest/config" />
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Two personalities, one config (ADR 0030):
//   - `vite`/`vite dev` (command "serve"): the dev harness — index.html/src/main.tsx
//     wraps ModuleStoreApp in a mock shell (src/devshell/DevShell.tsx) for local
//     development, since booth-design's real shell doesn't run here. Unaffected by
//     the `build.lib` config below — Vite's dev server ignores it.
//   - `vite build` (command "build"): builds the publishable library
//     (@projectbooth/module-store-ui) from src/index.ts, external-izing react/
//     react-dom so booth-design's own copies are used instead of bundling a second
//     one (a classic cause of "Invalid hook call" when two React copies mount in the
//     same page).
export default defineConfig(({ command }) => ({
  plugins: [react()],
  build:
    command === "build"
      ? {
          lib: {
            entry: fileURLToPath(new URL("./src/index.ts", import.meta.url)),
            formats: ["es"],
            fileName: "index",
          },
          rollupOptions: {
            external: ["react", "react-dom", "react/jsx-runtime"],
          },
        }
      : undefined,
  server: {
    proxy: {
      // Local dev only: booth-core's gateway would normally proxy
      // /modules/module-store/* to this service's own /api/* routes in a real
      // deployment. Pointed at BOOTH_MODULE_STORE_DEV_BACKEND when set, so `npm run
      // dev` can hit a locally running Go backend without CORS juggling.
      "/modules/module-store": {
        target: process.env.BOOTH_MODULE_STORE_DEV_BACKEND ?? "http://localhost:8080",
        changeOrigin: true,
        // Mimics the gateway's prefix-stripping: /modules/module-store/api/catalog
        // reaches the backend as /api/catalog.
        rewrite: (path) => path.replace(/^\/modules\/module-store/, ""),
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/setupTests.ts"],
  },
}));
