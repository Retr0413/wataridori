import { mountApplyDryRun } from "./features/applyDryRun.js";
import { createHistoryFeature } from "./features/history.js";
import { mountPromotePlan } from "./features/promote.js";
import { createStatusFeature } from "./features/status.js";
import { query, setBusy } from "./ui/dom.js";
import { Toast } from "./ui/toast.js";

function main(): void {
  const refresh = query<HTMLButtonElement>("#refresh");
  const toast = new Toast(query<HTMLElement>("#toast"));
  const status = createStatusFeature(toast);
  const history = createHistoryFeature();

  mountPromotePlan();
  mountApplyDryRun();

  refresh.addEventListener("click", () => {
    void reload(refresh, status.load, history.load);
  });

  void reload(refresh, status.load, history.load);
}

async function reload(
  button: HTMLButtonElement,
  loadStatus: () => Promise<void>,
  loadHistory: () => Promise<void>,
): Promise<void> {
  setBusy(button, true);
  try {
    await Promise.all([loadStatus(), loadHistory()]);
  } finally {
    setBusy(button, false);
  }
}

main();
