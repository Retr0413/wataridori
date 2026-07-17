import { useIsFetching, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Activity } from "./views/Activity";
import { Inventory } from "./views/Inventory";
import { Pipeline } from "./views/Pipeline";

const tabs = [
  { id: "pipeline", label: "Pipeline" },
  { id: "inventory", label: "Inventory" },
  { id: "activity", label: "Activity" },
] as const;

type TabId = (typeof tabs)[number]["id"];

export function App() {
  const [tab, setTab] = useState<TabId>("pipeline");
  const queryClient = useQueryClient();
  const fetching = useIsFetching() > 0;

  return (
    <>
      <header className="topbar">
        <div className="brand">
          <strong>Wataridori</strong>
          <span>Cloud Run deployments</span>
        </div>
        <div className="topbar-actions">
          {fetching && (
            <span className="muted" style={{ fontSize: 12 }}>
              Refreshing…
            </span>
          )}
          <button className="btn" onClick={() => void queryClient.invalidateQueries()}>
            Refresh
          </button>
        </div>
      </header>

      <nav className="tabs" role="tablist" aria-label="Views">
        {tabs.map((t) => (
          <button
            key={t.id}
            className="tab"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </nav>

      <main className="layout">
        {tab === "pipeline" && <Pipeline />}
        {tab === "inventory" && <Inventory />}
        {tab === "activity" && <Activity />}
      </main>
    </>
  );
}
