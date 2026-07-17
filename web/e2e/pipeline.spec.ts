import { expect, test } from "@playwright/test";

import { board, cell, expectBoardLoaded, tile } from "./helpers";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expectBoardLoaded(page);
});

test("columns follow the promotion chain, not the manifest map order", async ({ page }) => {
  // dev has no promoteFrom, prod promotes from dev, so dev must come first
  // even though ListEnvironments reads from an unordered Go map.
  const headers = board(page).locator("thead th").filter({ hasText: /dev|prod/ });
  await expect(headers).toHaveText([/^dev/, /^prod/]);
  await expect(headers.first()).toContainText("auto");
  await expect(headers.last()).toContainText("manual");
});

test("services are pivoted into one row each", async ({ page }) => {
  const names = board(page).locator("tbody tr td:first-child");
  await expect(names).toHaveText(["checkout-api", "web-frontend", "worker"]);
});

test("a fully promoted service shows the same digest in both environments", async ({ page }) => {
  const dev = cell(page, "checkout-api", "dev");
  const prod = cell(page, "checkout-api", "prod");
  await expect(dev).toContainText("a1b2c3d4e5f6");
  await expect(prod).toContainText("a1b2c3d4e5f6");
  await expect(dev).toContainText("in sync");
  await expect(prod).toContainText("in sync");
});

test("promote is offered only where the target manifest is behind", async ({ page }) => {
  // web-frontend: dev is on 9f8e…, prod still on 3c2b… .
  await expect(
    page.getByRole("button", { name: "Promote web-frontend from dev to prod" }),
  ).toBeVisible();

  // checkout-api and worker match their source, so no button anywhere else.
  await expect(page.getByRole("button", { name: /^Promote / })).toHaveCount(1);
});

test("drift and not-deployed are distinguished", async ({ page }) => {
  await expect(cell(page, "worker", "dev")).toContainText("drift");
  await expect(cell(page, "worker", "prod")).toContainText("not deployed");
  // The desired digest still shows for an undeployed service: the manifest
  // declares it, Cloud Run just does not have it yet.
  await expect(cell(page, "worker", "prod")).toContainText("5e4d3c2b1a09");
});

test("summary tiles count drift and pending promotions", async ({ page }) => {
  await expect(tile(page, "Services")).toHaveText("3");
  await expect(tile(page, "Drift")).toHaveText("1");
  await expect(tile(page, "Awaiting promote")).toHaveText("1");
});
