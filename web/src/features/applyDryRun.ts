import { procedures, rpc } from "../api/connect.js";
import type { ApplyResponse, ApplyServiceResult } from "../api/types.js";
import { inputValue, query, renderError, setBusy } from "../ui/dom.js";
import { shortImage } from "../ui/format.js";

export function mountApplyDryRun(): void {
  const form = query<HTMLFormElement>("#apply-form");
  const env = query<HTMLInputElement>("#apply-env");
  const service = query<HTMLInputElement>("#apply-service");
  const result = query<HTMLElement>("#apply-result");
  const submit = query<HTMLButtonElement>("button", form);

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    void dryRun({ env, service, result, submit });
  });
}

async function dryRun(nodes: {
  env: HTMLInputElement;
  service: HTMLInputElement;
  result: HTMLElement;
  submit: HTMLButtonElement;
}): Promise<void> {
  setBusy(nodes.submit, true);
  nodes.result.textContent = "Planning dry run...";
  try {
    const data = await rpc<ApplyResponse>(procedures.apply, {
      env: inputValue(nodes.env),
      service: inputValue(nodes.service),
      dryRun: true,
    });
    nodes.result.textContent = renderResult(data.services ?? []);
  } catch (error) {
    renderError(nodes.result, error);
  } finally {
    setBusy(nodes.submit, false);
  }
}

function renderResult(services: ApplyServiceResult[]): string {
  if (services.length === 0) {
    return "No services returned.";
  }
  return services.map((svc) => {
    const action = svc.inSync ? "up to date" : svc.actualImage ? "would update" : "would create";
    return `${svc.service ?? "-"}: ${action} ${shortImage(svc.desiredImage)}`;
  }).join("\n");
}
