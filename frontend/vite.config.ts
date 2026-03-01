import { defineConfig } from "vite";
import { resolve } from "path";

// Builds client-side assets (Alpine stores, SCSS, loading-store) → dist/public/
export default defineConfig({
  root: "client",
  build: {
    input: {
      main: resolve(__dirname, "client/main.ts"),
      loadingStore: resolve(__dirname, "client/stores/loading-store.js"),
    },
    emptyOutDir: true,
  },
});
