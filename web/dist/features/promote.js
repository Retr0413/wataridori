import { procedures, rpc } from "../api/connect.js";
import { inputValue, query, renderError, setBusy } from "../ui/dom.js";
import { shortImage } from "../ui/format.js";
export function mountPromotePlan() {
    const form = query("#promote-form");
    const from = query("#promote-from");
    const to = query("#promote-to");
    const service = query("#promote-service");
    const result = query("#promote-result");
    const submit = query("button", form);
    form.addEventListener("submit", (event) => {
        event.preventDefault();
        void plan({ from, to, service, result, submit });
    });
}
async function plan(nodes) {
    setBusy(nodes.submit, true);
    nodes.result.textContent = "Planning promotion...";
    try {
        const data = await rpc(procedures.planPromote, {
            from: inputValue(nodes.from),
            to: inputValue(nodes.to),
            service: inputValue(nodes.service),
        });
        nodes.result.textContent = renderPlan(data);
    }
    catch (error) {
        renderError(nodes.result, error);
    }
    finally {
        setBusy(nodes.submit, false);
    }
}
function renderPlan(data) {
    const items = data.items ?? [];
    if (items.length === 0) {
        return `${data.to || "target"} already matches ${data.from || "source"}.`;
    }
    return items.map(renderItem).join("\n");
}
function renderItem(item) {
    const copy = item.needsCopy ? " + image copy" : "";
    return `${item.service ?? "-"}: ${shortImage(item.oldImage)} -> ${shortImage(item.newImage)}${copy}`;
}
