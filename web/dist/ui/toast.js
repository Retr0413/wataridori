export class Toast {
    node;
    timer;
    constructor(node) {
        this.node = node;
    }
    show(message) {
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
