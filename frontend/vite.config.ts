import { defineConfig } from "vite";
import solidPlugin from "vite-plugin-solid";

export default defineConfig({
  plugins: [solidPlugin()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/v1": "http://localhost:4318",
    },
  },
  build: {
    target: "esnext",
  },
});
