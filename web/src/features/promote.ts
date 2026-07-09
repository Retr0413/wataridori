import { procedures, rpc } from "../api/connect.js";
import type { PlanPromoteResponse, PromoteItem } from "../api/types.js";
import { inputValue, query, renderError, setBusy } from "../ui/dom.js";
import { shortImage } from "../ui/format.js";

export function mountPromotePlan(): void {
  const form = query<HTMLFormElement>("#promote-form");
  const from = query<HTMLInputElement>("#promote-from");
  const to = query<HTMLInputElement>("#promote-to");
  const service = query<HTMLInputElement>("#promote-service");
  const result = query<HTMLElement>("#promote-result");
  const submit = query<HTMLButtonElement>("button", form);

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    void plan({ from, to, service, result, submit });
  });
}

async function plan(nodes: {
  from: HTMLInputElement;
  to: HTMLInputElement;
  service: HTMLInputElement;
  result: HTMLElement;
  submit: HTMLButtonElement;
}): Promise<void> {
  setBusy(nodes.submit, true);
  nodes.result.textContent = "Planning promotion...";
  try {
    const data = await rpc<PlanPromoteResponse>(procedures.planPromote, {
      from: inputValue(nodes.from),
      to: inputValue(nodes.to),
      service: inputValue(nodes.service),
    });
    nodes.result.textContent = renderPlan(data);
  } catch (error) {
    renderError(nodes.result, error);
  } finally {
    setBusy(nodes.submit, false);
  }
}

function renderPlan(data: PlanPromoteResponse): string {
  const items = data.items ?? [];
  if (items.length === 0) {
    return `${data.to || "target"} already matches ${data.from || "source"}.`;
  }
  return items.map(renderItem).join("\n");
}

function renderItem(item: PromoteItem): string {
  const copy = item.needsCopy ? " + image copy" : "";
  return `${item.service ?? "-"}: ${shortImage(item.oldImage)} -> ${shortImage(item.newImage)}${copy}`;
}
