import { defineConfig } from "@playwright/test";
const realAPI = process.env.E2E_REAL_API === "1";
const identity = realAPI && process.env.E2E_SUITE === "identity";
export default defineConfig({
  testDir: "./e2e",
  testMatch: realAPI
    ? identity
      ? "**/identity.spec.ts"
      : "**/postgres.spec.ts"
    : "**/*.spec.ts",
  testIgnore: realAPI ? [] : ["**/postgres.spec.ts", "**/identity.spec.ts"],
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI && !realAPI ? 1 : 0,
  use: {
    baseURL: "http://127.0.0.1:3000",
    trace: "retain-on-failure",
    launchOptions: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
      ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH }
      : {},
  },
  webServer: [
    ...(realAPI
      ? [
          {
            command: "go run ../cmd/testmail",
            url: "http://127.0.0.1:8026/healthz",
            reuseExistingServer: false,
            timeout: 60_000,
          },
        ]
      : []),
    ...(realAPI
      ? [
          {
            command: "go run ../cmd/api",
            url: "http://127.0.0.1:8080/healthz",
            reuseExistingServer: false,
            timeout: 60_000,
            env: { HTTP_ADDR: "127.0.0.1:8080" },
          },
        ]
      : []),
    {
      command: process.env.CI
        ? "npm run start -- --hostname 127.0.0.1"
        : "npm run dev -- --hostname 127.0.0.1",
      url: "http://127.0.0.1:3000",
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
    },
  ],
});
