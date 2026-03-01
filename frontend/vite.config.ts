import { defineConfig } from "vite";
import { resolve } from "path";

// Builds client-side assets (Alpine stores, SCSS) → dist/public/
export default defineConfig({
  root: "client",
  build: {
    outDir: "../dist/public",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: resolve(__dirname, "client/main.ts"),
      },
    },
  },
});
