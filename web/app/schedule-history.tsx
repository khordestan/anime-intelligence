"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";
import ProviderCredit from "./provider-credit";

type ScheduleRevision = {
  revision: number;
  provider?: string;
  scheduledAt?: string;
  releasedAt?: string;
  verifiedAt?: string;
};

type ScheduleHistoryPage = { items: ScheduleRevision[]; nextBeforeRevision?: number };

export default function ScheduleHistory({ client, animeID, number, track, revision }: {
  client: ApiClient;
  animeID: string;
  number: number;
  track: string;
  revision: number;
}) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<ScheduleRevision[]>([]);
  const [next, setNext] = useState<number | undefined>();
  const [retry, setRetry] = useState<number | undefined>();
  const [busy, setBusy] = useState(false);
  const requestRevision = useRef(0);

  useEffect(() => () => { requestRevision.current++; }, []);

  async function load(before: number) {
    const current = ++requestRevision.current;
    setOpen(true);
    setBusy(true);
    setRetry(undefined);
    const params = new URLSearchParams({ track, beforeRevision: String(before) });
    try {
      const page = await client.request<ScheduleHistoryPage>(
        "/anime/" + encodeURIComponent(animeID) + "/episodes/" + number + "/schedule-history?" + params,
      );
      if (current !== requestRevision.current) return;
      setItems(existing => before === revision ? page.items : [...existing, ...page.items]);
      setNext(page.nextBeforeRevision);
    } catch {
      if (current === requestRevision.current) setRetry(before);
    } finally {
      if (current === requestRevision.current) setBusy(false);
    }
  }

  const label = "Schedule history for episode " + number + " " + track;
  return (
    <div className="mt-1">
      <button type="button" aria-expanded={open} aria-label={open ? "Hide " + label.toLowerCase() : label}
        onClick={() => {
          if (open) {
            requestRevision.current++;
            setOpen(false);
            setBusy(false);
            setItems([]);
            setNext(undefined);
            setRetry(undefined);
          } else void load(revision);
        }}
        className="text-violet-200 underline">
        {open ? "Hide schedule history" : "Schedule history"}
      </button>
      {open && <section aria-label={label} className="mt-1 space-y-1 border-l border-white/10 pl-3">
        {items.length > 0 && <ol className="space-y-1">
          {items.map(item => (
            <li key={item.revision}>
              Revision {item.revision}
              {item.scheduledAt && <> · Scheduled {new Date(item.scheduledAt).toLocaleString()}</>}
              {item.releasedAt && <> · Confirmed {new Date(item.releasedAt).toLocaleString()}</>}
              {!item.scheduledAt && !item.releasedAt && <> · Time unknown</>}
              {item.provider && <> · Source: {item.provider}</>}
              {item.verifiedAt && <> · Observed {new Date(item.verifiedAt).toLocaleString()}</>}
            </li>
          ))}
        </ol>}
        {items.some(item => item.provider === "tvmaze") && <p><ProviderCredit provider="tvmaze" /></p>}
        {busy && <p role="status">Loading schedule history…</p>}
        {!busy && retry !== undefined && (
          <p role="status">
            Could not load schedule history. <button type="button" onClick={() => void load(retry)} className="underline">Retry</button>
          </p>
        )}
        {!busy && retry === undefined && items.length === 0 && <p>No earlier revisions are available.</p>}
        {!busy && retry === undefined && next !== undefined && (
          <button type="button" onClick={() => void load(next)} className="underline">Older revisions</button>
        )}
      </section>}
    </div>
  );
}
