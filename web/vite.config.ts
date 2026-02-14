import { defineConfig } from "vite";
import preact from "@preact/preset-vite";

export default defineConfig({
  plugins: [preact()],
  server: {
    port: 3000,
    proxy: {
      "/api": {
        target: "https://localhost:8443",
        secure: false,
        changeOrigin: true,
      },
      "/ws": {
        target: "wss://localhost:8443",
        secure: false,
        ws: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
