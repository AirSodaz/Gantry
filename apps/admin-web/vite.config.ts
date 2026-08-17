import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3001,
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
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
});
