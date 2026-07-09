import { escapeHTML } from "./dom.js";
const stateLabels = {
    "1": "in sync",
    "2": "drift",
    "3": "not deployed",
    SYNC_STATE_IN_SYNC: "in sync",
    SYNC_STATE_DRIFT: "drift",
    SYNC_STATE_NOT_DEPLOYED: "not deployed",
};
const actionLabels = {
    "1": "apply",
    "2": "promote",
    "3": "rollback",
    ACTION_APPLY: "apply",
    ACTION_PROMOTE: "promote",
    ACTION_ROLLBACK: "rollback",
};
export function syncStateLabel(state) {
    return stateLabels[String(state ?? "")] ?? String(state ?? "unknown").toLowerCase();
}
export function actionLabel(action) {
    return actionLabels[String(action ?? "")] ?? String(action ?? "-").toLowerCase();
}
export function shortDigest(value) {
    if (!value) {
        return "-";
    }
    const digest = value.includes("@") ? value.split("@").pop() : value;
    return digest?.replace(/^sha256:/, "").slice(0, 12) || "-";
}
export function shortImage(value) {
    if (!value) {
        return "-";
    }
    const [path, digest] = value.split("@");
    const name = path?.split("/").pop() || path || value;
    return digest ? `${name}@${shortDigest(digest)}` : name;
}
export function stateBadge(state) {
    const label = syncStateLabel(state);
    const kind = label === "in sync" ? "ok" : label === "drift" ? "bad" : "warn";
    return `<span class="badge ${kind}">${escapeHTML(label)}</span>`;
}
export function localTime(value) {
    if (!value) {
        return "-";
    }
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
        return value;
    }
    return date.toLocaleString();
}
