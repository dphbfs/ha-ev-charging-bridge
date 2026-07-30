import { resolve } from "node:path";
import { defineConfig } from "vite";

export default defineConfig({
  build: {
    rollupOptions: {
      input: {
        index: resolve("index.html"),
        session: resolve("session.html"),
        components: resolve("components.html"),
      },
    },
  },
});
