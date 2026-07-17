import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import { ToastProvider } from "./components/Toast";
import "./theme.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Status hits the Cloud Run API, so refetch on a timer rather than on
      // every window focus.
      refetchOnWindowFocus: false,
      refetchInterval: 30_000,
      retry: false,
      staleTime: 5_000,
    },
  },
});

const container = document.getElementById("root");
if (!container) {
  throw new Error("missing #root");
}

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <App />
      </ToastProvider>
    </QueryClientProvider>
  </StrictMode>,
);
