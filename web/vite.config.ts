import { defineConfig } from "vite";

// Writes one stable file name: the Go server embeds public/ as-is.
export default defineConfig({
  publicDir: false,
  build: {
    outDir: "../public",
    emptyOutDir: false,
    target: "es2022",
    lib: { entry: "src/main.ts", formats: ["iife"], name: "quake", fileName: () => "app.js" },
  },
});
