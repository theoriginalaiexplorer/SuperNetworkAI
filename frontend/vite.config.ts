import { defineConfig } from "vite";
import { resolve } from "path";

export default defineConfig({
  build: {
    outDir: "dist/public",
    rollupOptions: {
      input: resolve(__dirname, "client/main.ts"),
    },
    emptyOutDir: true,
  },
});
