import { defineConfig, devices } from "@playwright/test";

const port = 8137;

// The suite runs against the embedded production bundle served by the Go
// fake backend, not the vite dev server: that is what `wataridori serve`
// actually ships, and it exercises the real Connect handlers.
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [["github"], ["list"]] : [["list"]],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: "on-first-retry",
    // Pinned so rendered timestamps (toLocaleString) and screenshots do not
    // depend on the machine running the tests.
    locale: "en-US",
    timezoneId: "Asia/Tokyo",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 820 } },
    },
  ],
  webServer: {
    // web/dist must be current: assets.go embeds it. `npm run build` first.
    // Run from the repo root so the go module resolves.
    command: `go run ./web/e2e/fakeserver -addr 127.0.0.1:${port}`,
    cwd: "..",
    url: `http://127.0.0.1:${port}/`,
    reuseExistingServer: !process.env.CI,
    stdout: "pipe",
    stderr: "pipe",
    timeout: 120_000,
  },
});
