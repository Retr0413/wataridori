import { useQuery } from "@tanstack/react-query";
import { Fragment, useMemo, useState } from "react";

import { client } from "../api/client";
import { PromoteDialog } from "../components/PromoteDialog";
import { ServiceDrawer } from "../components/ServiceDrawer";
import { StateDot } from "../components/StateDot";
import type { Environment, ServiceStatus } from "../gen/wataridori/v1/wataridori_pb";
import { SyncState } from "../gen/wataridori/v1/wataridori_pb";
import { buildRows, countPromotable, isPromotable } from "../lib/board";
import type { BoardRow } from "../lib/board";
import { errorMessage, policyLabel, shortDigest, syncStateLabel } from "../lib/format";

interface PromoteTarget {
  service: string;
  from: string;
  to: string;
}

/**
 * Pipeline is the promotion board: one row per service, one column per
 * environment in promotion order, so a digest moving dev → prod reads as
 * left-to-right movement rather than as rows buried in a flat table.
 */
export function Pipeline() {
  const [detail, setDetail] = useState<ServiceStatus | null>(null);
  const [promoting, setPromoting] = useState<PromoteTarget | null>(null);

  const envsQuery = useQuery({
    queryKey: ["environments"],
    queryFn: () => client.listEnvironments({}),
  });
  const statusQuery = useQuery({
    queryKey: ["status"],
    queryFn: () => client.status({}),
  });

  const envs = useMemo(() => envsQuery.data?.environments ?? [], [envsQuery.data]);
  const rows = useMemo(() => buildRows(statusQuery.data?.services ?? []), [statusQuery.data]);

  const driftCount = useMemo(
    () => (statusQuery.data?.services ?? []).filter((s) => s.state === SyncState.DRIFT).length,
    [statusQuery.data],
  );
  const promotableCount = useMemo(() => countPromotable(rows, envs), [rows, envs]);

  if (envsQuery.isError) {
    return <p className="error">{errorMessage(envsQuery.error)}</p>;
  }
  if (statusQuery.isError) {
    return <p className="error">{errorMessage(statusQuery.error)}</p>;
  }

  return (
    <>
      <div className="tiles">
        <div className="tile">
          <div className="tile-label">Services</div>
          <div className="tile-value">{rows.length}</div>
        </div>
        <div className="tile">
          <div className="tile-label">Drift</div>
          <div className="tile-value">
            {driftCount > 0 && <span className="dot dot-warn" />}
            {driftCount}
          </div>
        </div>
        <div className="tile">
          <div className="tile-label">Awaiting promote</div>
          <div className="tile-value">{promotableCount}</div>
        </div>
      </div>

      <div className="panel panel-scroll">
        <table className="board">
          <thead>
            <tr>
              <th>Service</th>
              {envs.map((env, i) => (
                <Fragment key={env.name}>
                  {i > 0 && <th className="arrow" />}
                  <th>
                    {env.name} <span className="env-meta">· {policyLabel(env.policy)}</span>
                  </th>
                </Fragment>
              ))}
            </tr>
          </thead>
          <tbody>
            {(envsQuery.isPending || statusQuery.isPending) && (
              <tr>
                <td className="empty" colSpan={envs.length * 2}>
                  Loading…
                </td>
              </tr>
            )}
            {!statusQuery.isPending && rows.length === 0 && (
              <tr>
                <td className="empty" colSpan={Math.max(envs.length * 2, 2)}>
                  No services found in the manifest repository.
                </td>
              </tr>
            )}
            {rows.map((row) => (
              <tr key={row.service}>
                <td className="nowrap">{row.service}</td>
                {envs.map((env, i) => (
                  <Fragment key={env.name}>
                    {i > 0 && (
                      <td className={`arrow${isPromotable(row, env) ? " active" : ""}`}>→</td>
                    )}
                    <Cell
                      row={row}
                      env={env}
                      onOpen={setDetail}
                      onPromote={() =>
                        setPromoting({ service: row.service, from: env.promoteFrom, to: env.name })
                      }
                    />
                  </Fragment>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {detail && <ServiceDrawer status={detail} onClose={() => setDetail(null)} />}
      {promoting && (
        <PromoteDialog
          service={promoting.service}
          from={promoting.from}
          to={promoting.to}
          onClose={() => setPromoting(null)}
        />
      )}
    </>
  );
}

interface CellProps {
  row: BoardRow;
  env: Environment;
  onOpen: (status: ServiceStatus) => void;
  onPromote: () => void;
}

function Cell({ row, env, onOpen, onPromote }: CellProps) {
  const status = row.byEnv.get(env.name);
  if (!status) {
    return (
      <td>
        <span className="digest absent">Not declared</span>
      </td>
    );
  }

  const behind = isPromotable(row, env);
  return (
    <td>
      <div className="cell">
        <button
          className="row-head"
          onClick={() => onOpen(status)}
          style={buttonReset}
          aria-label={`${row.service} in ${env.name}`}
        >
          <div className={`digest${behind ? " stale" : ""}`}>
            {shortDigest(status.desiredDigest) || "-"}
          </div>
          <div className="substate">
            <StateDot state={status.state} />
            {syncStateLabel(status.state)}
            {status.state !== SyncState.NOT_DEPLOYED && ` · ${status.trafficPercent}%`}
          </div>
        </button>
        {behind && (
          <button
            className="btn btn-quiet nowrap"
            onClick={onPromote}
            aria-label={`Promote ${row.service} from ${env.promoteFrom} to ${env.name}`}
          >
            Promote
          </button>
        )}
      </div>
    </td>
  );
}

const buttonReset = {
  border: 0,
  padding: 0,
  background: "none",
  font: "inherit",
  textAlign: "left",
} as const;
