import { expect } from "@playwright/test";
import type { Locator, Page } from "@playwright/test";

export function board(page: Page): Locator {
  return page.locator("table.board");
}

/** expectBoardLoaded waits for the first row, so tests never race the RPCs. */
export async function expectBoardLoaded(page: Page): Promise<void> {
  await expect(board(page).locator("tbody tr").first()).toContainText("checkout-api");
}

/**
 * cell returns the board cell for one service in one environment, located
 * through the button's accessible name rather than column indexes (which
 * shift when the arrow columns move).
 */
export function cell(page: Page, service: string, env: string): Locator {
  return page.getByRole("button", { name: `${service} in ${env}`, exact: true });
}

/** tile returns the value element of a named summary tile. */
export function tile(page: Page, label: string): Locator {
  return page.locator(".tile").filter({ hasText: label }).locator(".tile-value");
}
