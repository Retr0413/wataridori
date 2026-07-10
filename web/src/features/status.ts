import { procedures, rpc } from "../api/connect.js";
import type { ServiceStatus, StatusResponse } from "../api/types.js";
import { inputValue, query, renderError, tableCell } from "../ui/dom.js";
import { shortImage, stateBadge, syncStateLabel } from "../ui/format.js";
import type { Toast } from "../ui/toast.js";

export interface StatusFeature {
  load(): Promise<void>;
}

export function createStatusFeature(toast: Toast): StatusFeature {
  const env = query<HTMLInputElement>("#status-env");
  const summary = query<HTMLElement>("#status-summary");
  const rows = query<HTMLElement>("#status-rows");

  async function load(): Promise<void> {
    summary.textContent = "Loading status...";
    try {
      const data = await rpc<StatusResponse>(procedures.status, { env: inputValue(env) });
      const services = data.services ?? [];
      renderSummary(summary, services);
      rows.innerHTML = renderRows(services);
      toast.show("Status loaded");
    } catch (error) {
      renderError(summary, error);
      rows.innerHTML = `<tr><td colspan="7" class="empty">Status unavailable.</td></tr>`;
    }
  }

  env.addEventListener("change", () => void load());
  return { load };
}

function renderSummary(node: HTMLElement, services: ServiceStatus[]): void {
  const driftCount = services.filter((svc) => syncStateLabel(svc.state) !== "in sync").length;
  node.textContent = services.length === 0
    ? "No services returned."
    : `${services.length} service${services.length === 1 ? "" : "s"} checked, ${driftCount} needing attention.`;
}

function renderRows(services: ServiceStatus[]): string {
  if (services.length === 0) {
    return `<tr><td colspan="7" class="empty">No services found.</td></tr>`;
  }
  return services.map((svc) => `
    <tr>
      ${tableCell(svc.env)}
      ${tableCell(svc.service)}
      ${tableCell(shortImage(svc.desiredImage), "wrap")}
      ${tableCell(shortImage(svc.actualImage), "wrap")}
      ${tableCell(svc.revision)}
      ${tableCell(svc.trafficPercent === undefined ? "-" : `${svc.trafficPercent}%`)}
      <td>${stateBadge(svc.state)}</td>
    </tr>
  `).join("");
}
