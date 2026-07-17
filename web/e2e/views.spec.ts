import { expect, test } from "@playwright/test";

test("the environment filter is populated from the server, never typed", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: "Activity" }).click();

  const select = page.getByLabel("Environment");
  // A select, not a text box: environment names are configuration.
  await expect(select).toHaveRole("combobox");
  await expect(select.locator("option")).toHaveText(["All", "dev", "prod"]);
});

test("inventory surfaces services no manifest declares", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: "Inventory" }).click();

  const rows = page.locator("table.table tbody tr");
  await expect(rows).toHaveCount(3);
  await expect(page.locator(".filters")).toContainText("3 services, 1 unmanaged");

  const legacy = rows.filter({ hasText: "legacy-cron" });
  await expect(legacy).toContainText("unmanaged");
});

test("inventory filters by environment", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: "Inventory" }).click();
  await page.getByLabel("Environment").selectOption("prod");

  const rows = page.locator("table.table tbody tr");
  await expect(rows).toHaveCount(1);
  await expect(rows).toContainText("worker");
});

test("activity renders recorded operations newest first", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: "Activity" }).click();

  const actions = page.locator("table.table tbody tr td:nth-child(2)");
  await expect(actions).toHaveText(["promote", "apply", "rollback"]);
});

test("activity decodes protobuf timestamps into local time", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: "Activity" }).click();

  // fixedTime is 2026-07-17T09:30:00Z and the suite pins Asia/Tokyo (+9) and
  // en-US, so the entry must render as 6:30 PM on the 17th.
  const first = page.locator("table.table tbody tr").first();
  await expect(first.locator("td").first()).toHaveText("7/17/2026, 6:30:00 PM");
});

test("activity filters by environment", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("tab", { name: "Activity" }).click();
  await page.getByLabel("Environment").selectOption("dev");

  const rows = page.locator("table.table tbody tr");
  await expect(rows).toHaveCount(1);
  await expect(rows).toContainText("web-frontend");
});
