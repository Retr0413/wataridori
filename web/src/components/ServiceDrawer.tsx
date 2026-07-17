import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { client } from "../api/client";
import type { ServiceStatus } from "../gen/wataridori/v1/wataridori_pb";
import { SyncState } from "../gen/wataridori/v1/wataridori_pb";
import { errorMessage, shortDigest, syncStateLabel } from "../lib/format";
import { useEscape } from "../lib/useEscape";
import { StateDot } from "./StateDot";
import { useToast } from "./Toast";

interface Props {
  status: ServiceStatus;
  onClose: () => void;
}

/**
 * ServiceDrawer is the detail view for one service in one environment: the
 * desired/actual comparison, why Cloud Run is or is not ready, deep links out
 * to the Console, and the rollback control.
 */
export function ServiceDrawer({ status, onClose }: Props) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [confirmingRollback, setConfirmingRollback] = useState(false);

  const rollbackPlan = useQuery({
    queryKey: ["planRollback", status.env, status.service],
    queryFn: () => client.planRollback({ env: status.env, service: status.service }),
    // Rollback only makes sense once something is deployed.
    enabled: status.state !== SyncState.NOT_DEPLOYED,
    retry: false,
  });

  const rollback = useMutation({
    mutationFn: () => client.executeRollback({ env: status.env, service: status.service }),
    onSuccess: async (res) => {
      const item = res.items[0];
      toast(`Rolled ${status.service} back to ${item?.targetRevision ?? "previous revision"}`);
      await queryClient.invalidateQueries();
      setConfirmingRollback(false);
    },
    onError: (error) => {
      toast(errorMessage(error), "bad");
      setConfirmingRollback(false);
    },
  });

  const target = rollbackPlan.data?.items[0];

  // Escape must not abandon an in-flight rollback.
  useEscape(onClose, !rollback.isPending);

  return (
    <>
      <button className="scrim" onClick={onClose} aria-label="Close details" />
      <aside className="drawer" aria-label={`${status.service} in ${status.env}`}>
        <div className="drawer-head">
          <div>
            <div className="drawer-title">{status.service}</div>
            <div className="muted" style={{ fontSize: 12 }}>
              {status.env}
            </div>
          </div>
          <button className="btn" onClick={onClose}>
            Close
          </button>
        </div>

        <div className="drawer-body">
          <section className="drawer-section">
            <div className="section-label">State</div>
            <div className="substate" style={{ fontSize: 13 }}>
              <StateDot state={status.state} />
              {syncStateLabel(status.state)}
              {status.state !== SyncState.NOT_DEPLOYED && ` · ${status.trafficPercent}% traffic`}
            </div>
            {status.readyMessage && (
              <p className="muted" style={{ marginTop: 6, fontSize: 12 }}>
                {status.readyMessage}
              </p>
            )}
          </section>

          <section className="drawer-section">
            <div className="section-label">Image</div>
            <dl className="kv">
              <dt>Desired</dt>
              <dd className="mono">{shortDigest(status.desiredDigest) || "-"}</dd>
              <dt>Actual</dt>
              <dd className="mono">{shortDigest(status.actualDigest) || "not deployed"}</dd>
              <dt>Revision</dt>
              <dd className="mono">{status.revision || "-"}</dd>
            </dl>
          </section>

          {(status.url || status.consoleUrl) && (
            <section className="drawer-section">
              <div className="section-label">Links</div>
              <div className="links">
                {status.url && (
                  <a className="link-btn" href={status.url} target="_blank" rel="noreferrer">
                    Service URL
                  </a>
                )}
                {status.consoleUrl && (
                  <a className="link-btn" href={status.consoleUrl} target="_blank" rel="noreferrer">
                    Cloud Console
                  </a>
                )}
              </div>
            </section>
          )}

          <section className="drawer-section">
            <div className="section-label">Rollback</div>
            {status.state === SyncState.NOT_DEPLOYED ? (
              <p className="muted" style={{ fontSize: 12 }}>
                Nothing deployed to roll back to.
              </p>
            ) : rollbackPlan.isPending ? (
              <p className="muted" style={{ fontSize: 12 }}>
                Looking for a previous revision…
              </p>
            ) : rollbackPlan.isError ? (
              <p className="muted" style={{ fontSize: 12 }}>
                {errorMessage(rollbackPlan.error)}
              </p>
            ) : !target ? (
              <p className="muted" style={{ fontSize: 12 }}>
                No previous revision to roll back to.
              </p>
            ) : confirmingRollback ? (
              <>
                <div className="diff">
                  <div className="diff-del">
                    <span className="sign">−</span> {target.currentRevision}
                  </div>
                  <div className="diff-add">
                    <span className="sign">+</span> {target.targetRevision}
                  </div>
                </div>
                <p className="note">Sends 100% of traffic to {target.targetRevision}.</p>
                <div className="modal-actions">
                  <button
                    className="btn"
                    onClick={() => setConfirmingRollback(false)}
                    disabled={rollback.isPending}
                  >
                    Cancel
                  </button>
                  <button
                    className="btn btn-danger"
                    onClick={() => rollback.mutate()}
                    disabled={rollback.isPending}
                  >
                    {rollback.isPending ? "Rolling back…" : "Roll back"}
                  </button>
                </div>
              </>
            ) : (
              <>
                <p className="muted" style={{ fontSize: 12, marginBottom: 8 }}>
                  Previous revision: <span className="mono">{target.targetRevision}</span>
                </p>
                <button className="btn btn-danger" onClick={() => setConfirmingRollback(true)}>
                  Roll back
                </button>
              </>
            )}
          </section>
        </div>
      </aside>
    </>
  );
}
