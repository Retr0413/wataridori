export function query<T extends Element>(selector: string, root: ParentNode = document): T {
  const node = root.querySelector<T>(selector);
  if (!node) {
    throw new Error(`missing element: ${selector}`);
  }
  return node;
}

export function inputValue(node: HTMLInputElement): string {
  return node.value.trim();
}

export function setBusy(node: HTMLButtonElement, busy: boolean): void {
  node.disabled = busy;
}

export function renderError(target: HTMLElement, error: unknown): void {
  target.textContent = `Error: ${errorMessage(error)}`;
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function escapeHTML(value: unknown): string {
  return displayText(value).replace(/[&<>"']/g, (ch) => {
    const escaped: Record<string, string> = {
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      "\"": "&quot;",
      "'": "&#39;",
    };
    return escaped[ch] ?? ch;
  });
}

export function displayText(value: unknown): string {
  if (value === undefined || value === null || value === "") {
    return "-";
  }
  return String(value);
}

export function tableCell(value: unknown, className = ""): string {
  return `<td${className ? ` class="${className}"` : ""}>${escapeHTML(value)}</td>`;
}
