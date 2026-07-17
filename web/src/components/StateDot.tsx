import { InventoryState, SyncState } from "../gen/wataridori/v1/wataridori_pb";

const syncTone: Record<number, string> = {
  [SyncState.IN_SYNC]: "dot-ok",
  [SyncState.DRIFT]: "dot-warn",
  [SyncState.NOT_DEPLOYED]: "dot-muted",
};

const inventoryTone: Record<number, string> = {
  [InventoryState.IN_SYNC]: "dot-ok",
  [InventoryState.DRIFT]: "dot-warn",
  [InventoryState.NOT_DEPLOYED]: "dot-muted",
  [InventoryState.UNMANAGED]: "dot-muted",
};

/** StateDot is the colour-coded marker used instead of a tinted badge, so a
 *  dense board stays readable. */
export function StateDot({ state }: { state: SyncState }) {
  return <span className={`dot ${syncTone[state] ?? "dot-muted"}`} />;
}

export function InventoryDot({ state }: { state: InventoryState }) {
  return <span className={`dot ${inventoryTone[state] ?? "dot-muted"}`} />;
}
