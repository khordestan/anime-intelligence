"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Cycle = { cycle: number; count: number; total: number | null; numbers: number[] | null; mode: string; rating: number | null; archivedAt: string };
type History = { currentCycle: number; items: Cycle[]; nextBefore: number };
export default function RewatchManager({ client, id, title, completed, disabled, onSaved, onError }: {
  client: ApiClient; id: string; title: string; completed: boolean; disabled: boolean;
  onSaved: () => Promise<void>; onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [history, setHistory] = useState<History | null>(null);
  const [confirm, setConfirm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const alive = useRef(true), revision = useRef(0), inFlight = useRef(false);
  useEffect(() => { alive.current = true; return () => { alive.current = false; revision.current++; }; }, []);
  async function load(before = 0) {
    if (inFlight.current) return;
    inFlight.current = true;
    const current = ++revision.current;
    setOpen(true); setBusy(true); setConfirm(false); setError("");
    try {
      const next = await client.request<History>("/tracker/" + id + "/rewatches" + (before ? "?before=" + before : ""));
      if (alive.current && current === revision.current) setHistory(next);
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not load viewing history. Try again."); onError(err); }
    } finally {
      if (alive.current && current === revision.current) { setBusy(false); inFlight.current = false; }
    }
  }
  async function start() {
    if (inFlight.current || !history) return;
    inFlight.current = true;
    const current = ++revision.current;
    setBusy(true); setError("");
    try {
      await client.request("/tracker/" + id + "/rewatches", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ expectedCycle: history.currentCycle }),
      });
      if (!alive.current || current !== revision.current) return;
      await onSaved();
      if (alive.current && current === revision.current) { setConfirm(false); setOpen(false); }
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not start rewatch. Refresh library and history before retrying."); setConfirm(false); onError(err); }
    } finally {
      if (alive.current && current === revision.current) { setBusy(false); inFlight.current = false; }
    }
  }
  return <div className="mt-4 border-t border-white/10 pt-3">
    {!open ? <button disabled={disabled} aria-label={"Viewing history " + title} onClick={() => void load()} className="text-xs text-violet-200 underline disabled:opacity-50">Viewing history and rewatches</button> :
      <section aria-label={"Viewing history " + title}>
        <p className="text-xs text-slate-400">Current viewing pass: {history?.currentCycle ?? "…"}. History contains passes you declared completed before explicitly starting another.</p>
        {busy && <p role="status" className="mt-2 text-xs">Loading viewing history…</p>}
        {error && <p role="alert" className="mt-2 text-xs text-rose-200">{error}</p>}
        {!busy && history?.items.length === 0 && <p className="mt-2 text-xs text-slate-400">No archived viewing passes yet.</p>}
        <ul className="mt-2 space-y-2" aria-label="Completed viewing passes">
          {history?.items.map(cycle => <li key={cycle.cycle} className="rounded-lg bg-white/5 p-2 text-xs text-slate-400">
            <p>Pass {cycle.cycle} · {cycle.count}{cycle.total === null ? " episodes (total unknown)" : " / " + cycle.total + " episodes"} · Rating {cycle.rating ?? "not set"}</p>
            {cycle.mode === "EXACT" ? <p>Exact watched episodes: {cycle.numbers?.join(", ") || "none"}</p> : <p>Count mode; episode identities were not recorded.</p>}
            <p>Archived {new Date(cycle.archivedAt).toLocaleString()}</p>
          </li>)}
        </ul>
        <div className="mt-3 flex flex-wrap gap-3">
          <button disabled={busy || disabled} onClick={() => void load()} className="text-xs underline disabled:opacity-50">Refresh viewing history</button>
          {!!history?.nextBefore && <button disabled={busy || disabled} onClick={() => void load(history.nextBefore)} className="text-xs underline disabled:opacity-50">Older viewing passes</button>}
          <button disabled={busy} onClick={() => { revision.current++; setOpen(false); setConfirm(false); }} className="text-xs underline">Close viewing history</button>
        </div>
        {history && completed && history.currentCycle < 1000 && !confirm && <button disabled={busy || disabled} onClick={() => setConfirm(true)} className="mt-3 text-xs text-violet-200 underline disabled:opacity-50">Start rewatch</button>}
        {confirm && <div role="group" aria-label={"Confirm rewatch " + title} className="mt-3 text-xs text-slate-300">
          <p>Archive pass {history?.currentCycle} and start the next pass at zero watched episodes? Your rating and follows stay in place.</p>
          <div className="mt-2 flex gap-3">
            <button disabled={busy || disabled} onClick={() => void start()} className="text-violet-200 underline disabled:opacity-50">Confirm start rewatch</button>
            <button disabled={busy} onClick={() => setConfirm(false)} className="underline">Cancel rewatch</button>
          </div>
        </div>}
        {!completed && <p className="mt-3 text-xs text-slate-500">Mark the current pass completed before starting another.</p>}
        {history && history.currentCycle >= 1000 && <p className="mt-3 text-xs text-slate-500">The viewing-pass limit has been reached.</p>}
      </section>}
  </div>;
}
