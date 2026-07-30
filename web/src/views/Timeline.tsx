import { useQuery } from "@tanstack/react-query";
import { Fragment, useMemo, useState } from "react";

import { client } from "../api/client";
import type { Environment, TimelineEntry } from "../gen/wataridori/v1/wataridori_pb";
import { clockTime, errorMessage, localDate, localTime, relativeTime, shortDigest } from "../lib/format";
import { EnvFilter } from "./EnvFilter";

/**
 * Timeline reconstructs what happened from Cloud Run revisions rather than
 * from Wataridori's own operation log, so deploys made by a CI pipeline show
 * up too. Two things follow from that and drive the layout:
 *
 * - "Now serving" puts every environment's live revision side by side with its
 *   age, which is how a dev/prod difference becomes readable — a digest pair
 *   says nothing, "today" versus "28 days ago" says everything.
 * - The merged list below interleaves environments on one time axis, so the
 *   deploy that caused a drift is visible as an event rather than inferred.
 */
export function Timeline() {
  const [env, setEnv] = useState("");

  const envsQuery = useQuery({
    queryKey: ["environments"],
    queryFn: () => client.listEnvironments({}),
  });
  // Fetched unfiltered and narrowed in the client: "Now serving" must keep
  // showing every environment even while the list below is filtered to one.
  const timelineQuery = useQuery({
    queryKey: ["timeline"],
    queryFn: () => client.timeline({ limit: 20 }),
  });

  const envs = envsQuery.data?.environments ?? [];
  const entries = useMemo(() => timelineQuery.data?.entries ?? [], [timelineQuery.data]);
  const visible = useMemo(
    () => (env ? entries.filter((e) => e.env === env) : entries),
    [entries, env],
  );

  if (envsQuery.isError) {
    return <p className="error">{errorMessage(envsQuery.error)}</p>;
  }

  return (
    <>
      <NowServing envs={envs} entries={entries} />

      <div className="filters">
        <EnvFilter value={env} onChange={setEnv} />
        <span className="muted" style={{ fontSize: 12 }}>
          {visible.length} revisions · newest first
        </span>
      </div>

      {timelineQuery.isError && <p className="error">{errorMessage(timelineQuery.error)}</p>}

      <div className="panel panel-scroll">
        <table className="table">
          <thead>
            <tr>
              <th>Time</th>
              <th>Env</th>
              <th>Service</th>
              <th>Digest</th>
              <th>Revision</th>
              <th>Traffic</th>
              <th>State</th>
            </tr>
          </thead>
          <tbody>
            {timelineQuery.isPending && (
              <tr>
                <td className="empty" colSpan={7}>
                  Loading…
                </td>
              </tr>
            )}
            {timelineQuery.isSuccess && visible.length === 0 && (
              <tr>
                <td className="empty" colSpan={7}>
                  No Cloud Run revisions found for the managed services.
                </td>
              </tr>
            )}
            {groupByDate(visible).map(([date, rows]) => (
              <Fragment key={date}>
                <tr className="tl-date">
                  <td colSpan={7}>{date}</td>
                </tr>
                {rows.map((entry) => (
                  <Row key={`${entry.env}/${entry.revision}`} entry={entry} />
                ))}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function Row({ entry }: { entry: TimelineEntry }) {
  return (
    <tr className={entry.current ? "tl-current" : undefined}>
      <td className="nowrap" title={localTime(entry.createTime)}>
        {clockTime(entry.createTime)}
      </td>
      <td className="nowrap">
        <span className="chip">{entry.env}</span>
      </td>
      <td className="nowrap">
        {entry.consoleUrl ? (
          <a href={entry.consoleUrl} target="_blank" rel="noreferrer">
            {entry.service}
          </a>
        ) : (
          entry.service
        )}
      </td>
      <td className="mono">{shortDigest(entry.digest) || "-"}</td>
      <td className="mono muted">{entry.revision}</td>
      <td className="nowrap">{entry.trafficPercent > 0 ? `${entry.trafficPercent}%` : "-"}</td>
      <td className="nowrap">
        <span className="marks">
          {entry.current && <span className="badge badge-live">serving</span>}
          {entry.desired && <span className="badge badge-git">in Git</span>}
          {!entry.ready && <span className="badge badge-bad">not ready</span>}
        </span>
      </td>
    </tr>
  );
}

interface NowServingProps {
  envs: Environment[];
  entries: TimelineEntry[];
}

/**
 * NowServing lays the environments out in promotion order, each listing the
 * revision it is actually running. The "in Git" marker is what separates a
 * deliberate difference from a drift: an environment can be behind another and
 * still match its own manifest.
 */
function NowServing({ envs, entries }: NowServingProps) {
  const current = useMemo(() => entries.filter((e) => e.current), [entries]);
  if (envs.length === 0) return null;

  return (
    <div className="env-cards">
      {envs.map((env, i) => {
        const rows = current.filter((e) => e.env === env.name);
        return (
          <Fragment key={env.name}>
            {i > 0 && <div className="env-cards-arrow">→</div>}
            <section className="env-card">
              <header className="env-card-head">
                <strong>{env.name}</strong>
                <span className="muted">now serving</span>
              </header>
              {rows.length === 0 ? (
                <p className="muted" style={{ fontSize: 12 }}>
                  Nothing serving traffic.
                </p>
              ) : (
                rows.map((row) => (
                  <div className="env-card-row" key={row.service}>
                    <div className="env-card-service">{row.service}</div>
                    <div className="env-card-digest mono">
                      {shortDigest(row.digest) || "-"}
                      {row.desired ? (
                        <span className="badge badge-git">in Git</span>
                      ) : (
                        <span className="badge badge-warn">drift</span>
                      )}
                    </div>
                    <div className="muted" title={localTime(row.createTime)}>
                      {relativeTime(row.createTime)} · {row.revision}
                    </div>
                  </div>
                ))
              )}
            </section>
          </Fragment>
        );
      })}
    </div>
  );
}

/** groupByDate keeps the input order (newest first) within each day. */
function groupByDate(entries: TimelineEntry[]): [string, TimelineEntry[]][] {
  const groups: [string, TimelineEntry[]][] = [];
  for (const entry of entries) {
    const date = localDate(entry.createTime);
    const last = groups[groups.length - 1];
    if (last && last[0] === date) {
      last[1].push(entry);
    } else {
      groups.push([date, [entry]]);
    }
  }
  return groups;
}
