import { useEffect, useRef } from "react";

/**
 * useEscape runs onEscape when Escape is pressed, for as long as the calling
 * component is mounted. The callback is held in a ref so an inline arrow
 * function does not resubscribe the listener on every render.
 */
export function useEscape(onEscape: () => void, enabled = true): void {
  const callback = useRef(onEscape);
  callback.current = onEscape;

  useEffect(() => {
    if (!enabled) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") callback.current();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [enabled]);
}
