import { expect, test } from "@playwright/test";

import { cell, expectBoardLoaded } from "./helpers";

// These write the images used in the docs. The fake backend serves fixed data
// and the config pins locale/timezone, so re-running only changes the files
// when the UI actually changed — modulo a few pixels of antialiasing on
// rounded corners, which the renderer does not reproduce exactly.
const dir = "../docs/screenshots";

/** park moves the pointer off the UI so no stray hover state is captured. */
async function park(page: import("@playwright/test").Page): Promise<void> {
  await page.mouse.move(0, 0);
}

test.describe("screenshots", () => {
  test("pipeline board", async ({ page }) => {
    await page.goto("/");
    await expectBoardLoaded(page);
    await park(page);
    await page.screenshot({ path: `${dir}/pipeline.png` });
  });

  test("promote dialog", async ({ page }) => {
    await page.goto("/");
    await expectBoardLoaded(page);
    await page.getByRole("button", { name: "Promote web-frontend from dev to prod" }).click();
    // Wait for the plan itself, not just the dialog frame: screenshotting
    // while PlanPromote is still in flight captures "Planning…" instead.
    await expect(page.locator(".diff-add")).toBeVisible();
    await park(page);
    await page.screenshot({ path: `${dir}/promote.png` });
  });

  test("service drawer", async ({ page }) => {
    await page.goto("/");
    await expectBoardLoaded(page);
    await cell(page, "worker", "dev").click();
    const panel = page.getByRole("complementary", { name: "worker in dev" });
    // Likewise wait for PlanRollback to settle.
    await expect(panel.getByRole("button", { name: "Roll back" })).toBeVisible();
    await park(page);
    await page.screenshot({ path: `${dir}/drawer.png` });
  });

  test("inventory", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("tab", { name: "Inventory" }).click();
    await expect(page.locator("table.table tbody tr").first()).toContainText("checkout-api");
    await park(page);
    await page.screenshot({ path: `${dir}/inventory.png` });
  });

  test("activity", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("tab", { name: "Activity" }).click();
    await expect(page.locator("table.table tbody tr").first()).toContainText("promote");
    await park(page);
    await page.screenshot({ path: `${dir}/activity.png` });
  });
});
