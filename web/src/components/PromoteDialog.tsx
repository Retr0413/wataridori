import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { client } from "../api/client";
import { errorMessage, shortDigest } from "../lib/format";
import { useEscape } from "../lib/useEscape";
import { useToast } from "./Toast";

interface Props {
  service: string;
  from: string;
  to: string;
  onClose: () => void;
}

/**
 * PromoteDialog previews the digest move before committing it. The plan is
 * fetched fresh rather than passed in from the board, so the confirmation
 * shows what the server would actually do.
 */
export function PromoteDialog({ service, from, to, onClose }: Props) {
  const toast = useToast();
  const queryClient = useQueryClient();

  const plan = useQuery({
    queryKey: ["planPromote", from, to, service],
    queryFn: () => client.planPromote({ from, to, service }),
  });

  const execute = useMutation({
    mutationFn: () => client.executePromote({ from, to, service }),
    onSuccess: async (res) => {
      toast(`Promoted ${service} to ${to} (${res.commitId.slice(0, 7)})`);
      await queryClient.invalidateQueries();
      onClose();
    },
    onError: (error) => toast(errorMessage(error), "bad"),
  });

  const item = plan.data?.items.find((i) => i.service === service);
  const busy = execute.isPending;

  // Escape must not abandon an in-flight promotion.
  useEscape(onClose, !busy);

  return (
    <>
      <button className="scrim" onClick={onClose} aria-label="Close dialog" disabled={busy} />
      <div className="modal" role="dialog" aria-modal="true" aria-label={`Promote ${service}`}>
        <h2>Promote {service}</h2>
        <p className="modal-sub">
          {from} → {to}
        </p>

        {plan.isPending && <p className="muted">Planning…</p>}
        {plan.isError && <p className="error">{errorMessage(plan.error)}</p>}

        {plan.isSuccess && !item && (
          <p className="muted">{to} already matches {from}. Nothing to promote.</p>
        )}

        {item && (
          <>
            <div className="diff">
              <div className="diff-del">
                <span className="sign">−</span> {shortDigest(item.oldImage) || "not set"}
              </div>
              <div className="diff-add">
                <span className="sign">+</span> {shortDigest(item.newImage)}
              </div>
            </div>
            {item.needsCopy && (
              <p className="note">Image is copied into the {to} registry before the commit.</p>
            )}
          </>
        )}

        <div className="modal-actions">
          <button className="btn" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button
            className="btn btn-primary"
            onClick={() => execute.mutate()}
            disabled={busy || !item}
          >
            {busy ? "Promoting…" : "Promote"}
          </button>
        </div>
      </div>
    </>
  );
}
