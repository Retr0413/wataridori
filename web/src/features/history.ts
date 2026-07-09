import { procedures, rpc } from "../api/connect.js";
import type { HistoryEntry, HistoryResponse } from "../api/types.js";
import { errorMessage, inputValue, query, tableCell } from "../ui/dom.js";
import { actionLabel, localTime, shortDigest } from "../ui/format.js";

export interface HistoryFeature {
  load(): Promise<void>;
}

export function createHistoryFeature(): HistoryFeature {
  const env = query<HTMLInputElement>("#history-env");
  const rows = query<HTMLElement>("#history-rows");

  async function load(): Promise<void> {
    rows.innerHTML = `<tr><td colspan="7" class="empty">Loading history...</td></tr>`;
    try {
      const data = await rpc<HistoryResponse>(procedures.history, { env: inputValue(env), limit: 20 });
      rows.innerHTML = renderRows(data.entries ?? []);
    } catch (error) {
      rows.innerHTML = `<tr><td colspan="7" class="empty">${errorMessage(error)}</td></tr>`;
    }
  }

  env.addEventListener("change", () => void load());
  return { load };
}

function renderRows(entries: HistoryEntry[]): string {
  if (entries.length === 0) {
    return `<tr><td colspan="7" class="empty">No history records found.</td></tr>`;
  }
  return entries.map((entry) => {
    const detail = entry.detail ? JSON.stringify(entry.detail) : "";
    return `
      <tr>
        ${tableCell(localTime(entry.time))}
        ${tableCell(entry.actor)}
        ${tableCell(actionLabel(entry.action))}
        ${tableCell(entry.env)}
        ${tableCell(entry.service)}
        ${tableCell(shortDigest(entry.digest))}
        ${tableCell(detail, "wrap")}
      </tr>
    `;
  }).join("");
}
