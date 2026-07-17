export type SyncState = "SYNC_STATE_IN_SYNC" | "SYNC_STATE_DRIFT" | "SYNC_STATE_NOT_DEPLOYED" | number | string;

export type Action = "ACTION_APPLY" | "ACTION_PROMOTE" | "ACTION_ROLLBACK" | number | string;

export interface ServiceStatus {
  env?: string;
  service?: string;
  desiredImage?: string;
  desiredDigest?: string;
  actualImage?: string;
  actualDigest?: string;
  revision?: string;
  state?: SyncState;
  ready?: boolean;
  readyMessage?: string;
  trafficPercent?: number;
  url?: string;
  consoleUrl?: string;
}

export interface StatusResponse {
  services?: ServiceStatus[];
  drift?: boolean;
}

export type InventoryState =
  | "INVENTORY_STATE_IN_SYNC"
  | "INVENTORY_STATE_DRIFT"
  | "INVENTORY_STATE_NOT_DEPLOYED"
  | "INVENTORY_STATE_UNMANAGED"
  | number
  | string;

export interface InventoryItem {
  env?: string;
  project?: string;
  region?: string;
  service?: string;
  managed?: boolean;
  desiredImage?: string;
  desiredDigest?: string;
  actualImage?: string;
  actualDigest?: string;
  revision?: string;
  trafficPercent?: number;
  ready?: boolean;
  readyMessage?: string;
  url?: string;
  consoleUrl?: string;
  state?: InventoryState;
}

export interface InventoryResponse {
  items?: InventoryItem[];
}

export interface HistoryEntry {
  id?: number;
  time?: string;
  actor?: string;
  action?: Action;
  env?: string;
  service?: string;
  digest?: string;
  detail?: Record<string, string>;
}

export interface HistoryResponse {
  entries?: HistoryEntry[];
}

export interface PromoteItem {
  service?: string;
  fromImage?: string;
  oldImage?: string;
  newImage?: string;
  needsCopy?: boolean;
}

export interface PlanPromoteResponse {
  from?: string;
  to?: string;
  items?: PromoteItem[];
}

export interface ApplyServiceResult {
  service?: string;
  desiredImage?: string;
  actualImage?: string;
  revision?: string;
  url?: string;
  inSync?: boolean;
}

export interface ApplyResponse {
  env?: string;
  dryRun?: boolean;
  services?: ApplyServiceResult[];
}
