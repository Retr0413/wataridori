export function query(selector, root = document) {
    const node = root.querySelector(selector);
    if (!node) {
        throw new Error(`missing element: ${selector}`);
    }
    return node;
}
export function inputValue(node) {
    return node.value.trim();
}
export function setBusy(node, busy) {
    node.disabled = busy;
}
export function renderError(target, error) {
    target.textContent = `Error: ${errorMessage(error)}`;
}
export function errorMessage(error) {
    return error instanceof Error ? error.message : String(error);
}
export function escapeHTML(value) {
    return displayText(value).replace(/[&<>"']/g, (ch) => {
        const escaped = {
            "&": "&amp;",
            "<": "&lt;",
            ">": "&gt;",
            "\"": "&quot;",
            "'": "&#39;",
        };
        return escaped[ch] ?? ch;
    });
}
export function displayText(value) {
    if (value === undefined || value === null || value === "") {
        return "-";
    }
    return String(value);
}
export function tableCell(value, className = "") {
    return `<td${className ? ` class="${className}"` : ""}>${escapeHTML(value)}</td>`;
}
