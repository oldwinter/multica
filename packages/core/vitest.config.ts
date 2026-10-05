import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    execArgv: ["--no-experimental-webstorage"],
    globals: true,
    include: ["**/*.test.{ts,tsx}"],
    passWithNoTests: true,
  },
});
