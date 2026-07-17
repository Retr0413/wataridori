import type { Environment, ServiceStatus } from "../gen/wataridori/v1/wataridori_pb";

/** BoardRow is one service across every environment that declares it. */
export interface BoardRow {
  service: string;
  /** byEnv is keyed by environment name; absent means the environment's
   *  manifests do not declare this service at all. */
  byEnv: Map<string, ServiceStatus>;
}

/** buildRows pivots the flat status list into one row per service. */
export function buildRows(services: ServiceStatus[]): BoardRow[] {
  const rows = new Map<string, BoardRow>();
  for (const svc of services) {
    let row = rows.get(svc.service);
    if (!row) {
      row = { service: svc.service, byEnv: new Map() };
      rows.set(svc.service, row);
    }
    row.byEnv.set(svc.env, svc);
  }
  return [...rows.values()].sort((a, b) => a.service.localeCompare(b.service));
}

/**
 * isPromotable reports whether env's manifest is behind its promotion source
 * for this service — the digests are compared as declared in Git, not as
 * running on Cloud Run, because promotion moves manifests. PlanPromote
 * remains the authority; this only decides whether to offer the button.
 */
export function isPromotable(row: BoardRow, env: Environment): boolean {
  if (!env.promoteFrom) return false;
  const source = row.byEnv.get(env.promoteFrom);
  const target = row.byEnv.get(env.name);
  if (!source?.desiredDigest || !target?.desiredDigest) return false;
  return source.desiredDigest !== target.desiredDigest;
}

/** countPromotable counts the (service, environment) pairs awaiting promotion. */
export function countPromotable(rows: BoardRow[], envs: Environment[]): number {
  let total = 0;
  for (const row of rows) {
    for (const env of envs) {
      if (isPromotable(row, env)) total++;
    }
  }
  return total;
}
