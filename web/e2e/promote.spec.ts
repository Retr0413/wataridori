import { expect, test } from "@playwright/test";

import { expectBoardLoaded } from "./helpers";

test.beforeEach(async ({ page }) => {
  await page.goto("/");
  await expectBoardLoaded(page);
  await page.getByRole("button", { name: "Promote web-frontend from dev to prod" }).click();
});

const dialog = (page: import("@playwright/test").Page) =>
  page.getByRole("dialog", { name: "Promote web-frontend" });

test("the dialog previews the digest move before anything is written", async ({ page }) => {
  const modal = dialog(page);
  await expect(modal).toBeVisible();
  await expect(modal).toContainText("dev → prod");
  // Old digest removed, new digest added.
  await expect(modal.locator(".diff-del")).toContainText("3c2b1a0f9e8d");
  await expect(modal.locator(".diff-add")).toContainText("9f8e7d6c5b4a");
});

test("the dialog warns when the promotion also copies the image", async ({ page }) => {
  // prod configures imageCopy, so PlanPromote sets needsCopy.
  await expect(dialog(page)).toContainText("Image is copied into the prod registry");
});

test("cancel closes without promoting", async ({ page }) => {
  await dialog(page).getByRole("button", { name: "Cancel" }).click();
  await expect(dialog(page)).toBeHidden();
  await expect(page.locator(".toast")).toBeHidden();
});

test("escape closes without promoting", async ({ page }) => {
  await page.keyboard.press("Escape");
  await expect(dialog(page)).toBeHidden();
  await expect(page.locator(".toast")).toBeHidden();
});

test("confirming promotes and reports the commit", async ({ page }) => {
  await dialog(page).getByRole("button", { name: "Promote", exact: true }).click();

  // The commit id comes back from ExecutePromote, truncated to 7 chars.
  await expect(page.locator(".toast")).toContainText("Promoted web-frontend to prod (abc1234)");
  await expect(dialog(page)).toBeHidden();
});
