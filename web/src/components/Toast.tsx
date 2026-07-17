import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";

type Tone = "ok" | "bad";

interface ToastMessage {
  text: string;
  tone: Tone;
  /** id forces a re-render when the same text is shown twice in a row. */
  id: number;
}

const ToastContext = createContext<(text: string, tone?: Tone) => void>(() => {});

/** useToast shows a transient status message in the corner. */
export function useToast(): (text: string, tone?: Tone) => void {
  return useContext(ToastContext);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [message, setMessage] = useState<ToastMessage | null>(null);

  const show = useCallback((text: string, tone: Tone = "ok") => {
    setMessage({ text, tone, id: Date.now() });
  }, []);

  useEffect(() => {
    if (!message) return;
    const timer = window.setTimeout(() => setMessage(null), 4000);
    return () => window.clearTimeout(timer);
  }, [message]);

  // show is stable, so this only re-creates when it has to.
  const value = useMemo(() => show, [show]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      {message && (
        <div className="toast" data-tone={message.tone} role="status" aria-live="polite">
          {message.text}
        </div>
      )}
    </ToastContext.Provider>
  );
}
