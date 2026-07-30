import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { timestampDate } from "@bufbuild/protobuf/wkt";

import { Action, Policy, SyncState } from "../gen/wataridori/v1/wataridori_pb";

/** shortDigest trims "sha256:" and keeps the leading 12 characters. */
export function shortDigest(value: string): string {
  if (!value) return "";
  const digest = value.includes("@") ? (value.split("@").pop() ?? "") : value;
  return digest.replace(/^sha256:/, "").slice(0, 12);
}

/** shortImage renders "repo/path@sha256:abcdef..." as "path@abcdef123456". */
export function shortImage(value: string): string {
  if (!value) return "";
  const [path = "", digest] = value.split("@");
  const name = path.split("/").pop() || path;
  return digest ? `${name}@${shortDigest(digest)}` : name;
}

export function syncStateLabel(state: SyncState): string {
  switch (state) {
    case SyncState.IN_SYNC:
      return "in sync";
    case SyncState.DRIFT:
      return "drift";
    case SyncState.NOT_DEPLOYED:
      return "not deployed";
    default:
      return "unknown";
  }
}

export function actionLabel(action: Action): string {
  switch (action) {
    case Action.APPLY:
      return "apply";
    case Action.PROMOTE:
      return "promote";
    case Action.ROLLBACK:
      return "rollback";
    default:
      return "-";
  }
}

export function policyLabel(policy: Policy): string {
  switch (policy) {
    case Policy.AUTO:
      return "auto";
    case Policy.MANUAL:
      return "manual";
    default:
      return "";
  }
}

export function localTime(ts: Timestamp | undefined): string {
  if (!ts) return "-";
  return timestampDate(ts).toLocaleString();
}

/** localDate is the day key used to group a timeline into date sections. */
export function localDate(ts: Timestamp | undefined): string {
  if (!ts) return "";
  return timestampDate(ts).toLocaleDateString();
}

/** clockTime drops the date, which the timeline's date heading already shows. */
export function clockTime(ts: Timestamp | undefined): string {
  if (!ts) return "-";
  return timestampDate(ts).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

const relative = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
];

/**
 * relativeTime renders "28 days ago". Age is what makes two environments
 * comparable at a glance — an absolute timestamp alone does not show that
 * prod has been sitting a month behind dev.
 */
export function relativeTime(ts: Timestamp | undefined): string {
  if (!ts) return "";
  const seconds = (timestampDate(ts).getTime() - Date.now()) / 1000;
  for (const [unit, size] of units) {
    if (Math.abs(seconds) >= size) {
      return relative.format(Math.round(seconds / size), unit);
    }
  }
  return relative.format(Math.round(seconds), "second");
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
