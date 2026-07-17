import { expect, test } from "@playwright/test";

import { cell, expectBoardLoaded } from "./helpers";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expectBoardLoaded(page);
});

const drawer = (page: import("@playwright/test").Page, name: string) =>
  page.getByRole("complementary", { name });

test("the drawer explains why a drifted service is not ready", async ({ page }) => {
  await cell(page, "worker", "dev").click();
  const panel = drawer(page, "worker in dev");

  await expect(panel).toContainText("drift · 100% traffic");
  // readyMessage is the field the old UI never rendered.
  await expect(panel).toContainText("Revision worker-00004 failed to become ready");
  // Desired and actual differ; that difference is the drift.
  await expect(panel).toContainText("5e4d3c2b1a09");
  await expect(panel).toContainText("3c2b1a0f9e8d");
  await expect(panel).toContainText("worker-00003");
});

test("the drawer deep links out to Cloud Run instead of rebuilding it", async ({ page }) => {
  await cell(page, "checkout-api", "dev").click();
  const panel = drawer(page, "checkout-api in dev");

  await expect(panel.getByRole("link", { name: "Service URL" })).toHaveAttribute(
    "href",
    "https://checkout-api-dev.a.run.app",
  );
  await expect(panel.getByRole("link", { name: "Cloud Console" })).toHaveAttribute(
    "href",
    /console\.cloud\.google\.com/,
  );
});

test("an undeployed service offers nothing to roll back to", async ({ page }) => {
  await cell(page, "worker", "prod").click();
  const panel = drawer(page, "worker in prod");

  await expect(panel).toContainText("not deployed");
  await expect(panel).toContainText("Nothing deployed to roll back to");
  await expect(panel.getByRole("button", { name: "Roll back" })).toHaveCount(0);
});

test("rollback requires confirming the revision switch", async ({ page }) => {
  await cell(page, "checkout-api", "prod").click();
  const panel = drawer(page, "checkout-api in prod");

  // PlanRollback names the previous revision before anything is offered.
  await expect(panel).toContainText("checkout-api-00002");
  await panel.getByRole("button", { name: "Roll back" }).click();

  await expect(panel.locator(".diff-del")).toContainText("checkout-api-00003");
  await expect(panel.locator(".diff-add")).toContainText("checkout-api-00002");
  await expect(panel).toContainText("Sends 100% of traffic to checkout-api-00002");

  await panel.getByRole("button", { name: "Roll back" }).click();
  await expect(page.locator(".toast")).toContainText("Rolled checkout-api back to checkout-api-00002");
});

test("escape closes the drawer", async ({ page }) => {
  await cell(page, "checkout-api", "dev").click();
  await expect(drawer(page, "checkout-api in dev")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(drawer(page, "checkout-api in dev")).toBeHidden();
});
