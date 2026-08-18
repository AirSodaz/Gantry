import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3002,
    proxy: {
      "/api": "http://localhost:8080",
      "/oidc": {
        target: "http://localhost:5556",
        rewrite: (path) => path.replace(/^\/oidc/, "/dex"),
      },
      "/dex": "http://localhost:5556",
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
    globals: true,
  },
});
