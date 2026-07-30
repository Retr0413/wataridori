import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";

async function openTimeline(page: Page): Promise<void> {
  await page.goto("/");
  await page.getByRole("tab", { name: "Timeline" }).click();
  await expect(page.locator(".env-card").first()).toContainText("dev");
}

test("now serving shows what each environment actually runs, in promotion order", async ({
  page,
}) => {
  await openTimeline(page);

  const cards = page.locator(".env-card");
  await expect(cards).toHaveCount(2);
  // dev before prod: the same left-to-right reading as the Pipeline board.
  await expect(cards.nth(0)).toContainText("dev");
  await expect(cards.nth(1)).toContainText("prod");

  // prod is a digest behind dev on web-frontend, yet still matches its own
  // manifest — "behind" and "drifted" have to stay distinguishable.
  const prod = cards.nth(1).locator(".env-card-row").filter({ hasText: "web-frontend" });
  await expect(prod).toContainText("in Git");
  await expect(prod).not.toContainText("drift");
});

test("a revision serving traffic that Git does not point at is marked as drift", async ({
  page,
}) => {
  await openTimeline(page);

  const worker = page.locator(".env-card").nth(0).locator(".env-card-row", { hasText: "worker" });
  await expect(worker).toContainText("drift");
});

test("the merged list interleaves environments newest first", async ({ page }) => {
  await openTimeline(page);

  const envs = page.locator("table.table tbody tr:not(.tl-date) td:nth-child(2)");
  await expect(envs).toHaveText(["dev", "dev", "prod", "dev", "dev", "prod", "dev"]);
});

test("the serving revision and the revision Git points at are both marked", async ({ page }) => {
  await openTimeline(page);

  const rows = page.locator("table.table tbody tr");
  // worker-00003 serves traffic; worker-00002 is what the manifest pins.
  await expect(rows.filter({ hasText: "worker-00003" })).toContainText("serving");
  await expect(rows.filter({ hasText: "worker-00003" })).not.toContainText("in Git");
  await expect(rows.filter({ hasText: "worker-00002" })).toContainText("in Git");
  await expect(rows.filter({ hasText: "worker-00002" })).not.toContainText("serving");
});

test("filtering narrows the list but keeps every environment in now serving", async ({ page }) => {
  await openTimeline(page);
  await page.getByLabel("Environment").selectOption("prod");

  const rows = page.locator("table.table tbody tr:not(.tl-date)");
  await expect(rows).toHaveCount(2);
  await expect(rows.first()).toContainText("checkout-api-00004");
  // The comparison is the point of the view, so it must survive the filter.
  await expect(page.locator(".env-card")).toHaveCount(2);
});

test("timeline groups revisions under local dates", async ({ page }) => {
  await openTimeline(page);

  // fixedTime is 2026-07-17T09:30:00Z and the suite pins Asia/Tokyo (+9), so
  // the newest entry (90 minutes earlier) falls on the 17th local time.
  await expect(page.locator("tr.tl-date").first()).toHaveText("7/17/2026");
});
