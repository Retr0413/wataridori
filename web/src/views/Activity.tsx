import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { client } from "../api/client";
import { actionLabel, errorMessage, localTime, shortDigest } from "../lib/format";
import { EnvFilter } from "./EnvFilter";

/** Activity lists recorded apply / promote / rollback operations. */
export function Activity() {
  const [env, setEnv] = useState("");
  const query = useQuery({
    queryKey: ["history", env],
    queryFn: () => client.history({ env, limit: 50 }),
  });

  const entries = query.data?.entries ?? [];

  return (
    <>
      <div className="filters">
        <EnvFilter value={env} onChange={setEnv} />
      </div>

      {query.isError && <p className="error">{errorMessage(query.error)}</p>}

      <div className="panel panel-scroll">
        <table className="table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Action</th>
              <th>Env</th>
              <th>Service</th>
              <th>Digest</th>
              <th>Actor</th>
              <th>Detail</th>
            </tr>
          </thead>
          <tbody>
            {query.isPending && (
              <tr>
                <td className="empty" colSpan={7}>
                  Loading…
                </td>
              </tr>
            )}
            {query.isSuccess && entries.length === 0 && (
              <tr>
                <td className="empty" colSpan={7}>
                  No operations recorded yet.
                </td>
              </tr>
            )}
            {entries.map((entry) => (
              <tr key={String(entry.id)}>
                <td className="nowrap">{localTime(entry.time)}</td>
                <td className="nowrap">{actionLabel(entry.action)}</td>
                <td className="nowrap">{entry.env || "-"}</td>
                <td className="nowrap">{entry.service || "-"}</td>
                <td className="mono">{shortDigest(entry.digest) || "-"}</td>
                <td className="nowrap">{entry.actor || "-"}</td>
                <td className="muted">{formatDetail(entry.detail)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function formatDetail(detail: Record<string, string>): string {
  const parts = Object.entries(detail).map(([key, value]) => `${key}=${value}`);
  return parts.length > 0 ? parts.join(" ") : "-";
}
