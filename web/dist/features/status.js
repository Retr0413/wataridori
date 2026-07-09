import { procedures, rpc } from "../api/connect.js";
import { inputValue, query, renderError, tableCell } from "../ui/dom.js";
import { shortImage, stateBadge, syncStateLabel } from "../ui/format.js";
export function createStatusFeature(toast) {
    const env = query("#status-env");
    const summary = query("#status-summary");
    const rows = query("#status-rows");
    async function load() {
        summary.textContent = "Loading status...";
        try {
            const data = await rpc(procedures.status, { env: inputValue(env) });
            const services = data.services ?? [];
            renderSummary(summary, services);
            rows.innerHTML = renderRows(services);
            toast.show("Status loaded");
        }
        catch (error) {
            renderError(summary, error);
            rows.innerHTML = `<tr><td colspan="7" class="empty">Status unavailable.</td></tr>`;
        }
    }
    env.addEventListener("change", () => void load());
    return { load };
}
function renderSummary(node, services) {
    const driftCount = services.filter((svc) => syncStateLabel(svc.state) !== "in sync").length;
    node.textContent = services.length === 0
        ? "No services returned."
        : `${services.length} service${services.length === 1 ? "" : "s"} checked, ${driftCount} needing attention.`;
}
function renderRows(services) {
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
