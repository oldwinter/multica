import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    execArgv: ["--no-experimental-webstorage"],
    environment: "jsdom",
    globals: true,
    setupFiles: ["./test/setup.ts"],
    include: ["**/*.test.{ts,tsx}"],
  },
});
