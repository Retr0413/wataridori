import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { client } from "../api/client";
import { InventoryDot } from "../components/StateDot";
import { InventoryState } from "../gen/wataridori/v1/wataridori_pb";
import { errorMessage, shortDigest } from "../lib/format";
import { EnvFilter } from "./EnvFilter";

const stateLabels: Record<number, string> = {
  [InventoryState.IN_SYNC]: "in sync",
  [InventoryState.DRIFT]: "drift",
  [InventoryState.NOT_DEPLOYED]: "not deployed",
  [InventoryState.UNMANAGED]: "unmanaged",
};

/**
 * Inventory lists every Cloud Run service in the configured projects,
 * including ones no manifest declares — the first step before importing them.
 */
export function Inventory() {
  const [env, setEnv] = useState("");
  const query = useQuery({
    queryKey: ["inventory", env],
    queryFn: () => client.inventory({ env }),
  });

  const items = query.data?.items ?? [];
  const unmanaged = items.filter((i) => !i.managed).length;

  return (
    <>
      <div className="filters">
        <EnvFilter value={env} onChange={setEnv} />
        {query.isSuccess && (
          <span className="muted" style={{ fontSize: 12 }}>
            {items.length} services, {unmanaged} unmanaged
          </span>
        )}
      </div>

      {query.isError && <p className="error">{errorMessage(query.error)}</p>}

      <div className="panel panel-scroll">
        <table className="table">
          <thead>
            <tr>
              <th>Env</th>
              <th>Service</th>
              <th>Managed</th>
              <th>Desired</th>
              <th>Actual</th>
              <th>Revision</th>
              <th>Traffic</th>
              <th>State</th>
              <th>Links</th>
            </tr>
          </thead>
          <tbody>
            {query.isPending && (
              <tr>
                <td className="empty" colSpan={9}>
                  Loading…
                </td>
              </tr>
            )}
            {query.isSuccess && items.length === 0 && (
              <tr>
                <td className="empty" colSpan={9}>
                  No Cloud Run services found.
                </td>
              </tr>
            )}
            {items.map((item) => (
              <tr key={`${item.env}/${item.service}`}>
                <td className="nowrap">{item.env}</td>
                <td className="nowrap">
                  {item.service}
                  {/* The manifest identity, shown only when it differs, so a
                      Cloud Run name like api-prod is traceable to its row. */}
                  {item.manifestService && item.manifestService !== item.service && (
                    <span className="muted"> · {item.manifestService}</span>
                  )}
                </td>
                <td className="muted nowrap">{item.managed ? "managed" : "unmanaged"}</td>
                <td className="mono">{shortDigest(item.desiredDigest) || "-"}</td>
                <td className="mono">{shortDigest(item.actualDigest) || "-"}</td>
                <td className="mono nowrap">{item.revision || "-"}</td>
                <td className="nowrap">
                  {item.state === InventoryState.NOT_DEPLOYED ? "-" : `${item.trafficPercent}%`}
                </td>
                <td className="nowrap">
                  <span className="substate">
                    <InventoryDot state={item.state} />
                    {stateLabels[item.state] ?? "unknown"}
                  </span>
                </td>
                <td className="nowrap">
                  {item.consoleUrl ? (
                    <a href={item.consoleUrl} target="_blank" rel="noreferrer">
                      Console
                    </a>
                  ) : (
                    <span className="muted">-</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
