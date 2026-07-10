export class Toast {
  private timer: number | undefined;

  constructor(private readonly node: HTMLElement) {}

  show(message: string): void {
    this.node.textContent = message;
    this.node.classList.add("visible");
    if (this.timer !== undefined) {
      window.clearTimeout(this.timer);
    }
    this.timer = window.setTimeout(() => {
      this.node.classList.remove("visible");
    }, 3200);
  }
}
