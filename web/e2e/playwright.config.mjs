import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.mjs",
  globalSetup: "./global-setup.mjs",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 5_000 },
  reporter: [["list"]],
  outputDir: "./.results",
  use: { browserName: "chromium", viewport: { width: 1280, height: 800 } },
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
});
