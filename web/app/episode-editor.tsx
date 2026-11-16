"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";
import { parseEpisodeNumbers } from "../lib/episodes";

type Progress = { watchCycle?: number; mode: "COUNT" | "EXACT"; count: number; numbers: number[] | null; total: number | null };
export default function EpisodeEditor({ client, id, title, disabled, onSaved, onError }: {
  client: ApiClient; id: string; title: string; disabled: boolean;
  onSaved: () => Promise<void>; onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const alive = useRef(true);
  const revision = useRef(0);
  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; revision.current++; };
  }, []);
  async function load() {
    const current = ++revision.current;
    setOpen(true); setBusy(true); setError(""); setProgress(null);
    try {
      const next = await client.request<Progress>("/tracker/" + id + "/episodes");
      if (!alive.current || current !== revision.current) return;
      setProgress(next);
      setValue(next.numbers?.join(", ") ?? "");
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not load episode progress. Try again."); onError(err); }
    } finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  async function save() {
    let numbers: number[];
    try {
      numbers = parseEpisodeNumbers(value);
      if (progress?.total != null && numbers.some(n => n > progress.total!)) throw new Error("An episode exceeds the known total.");
    } catch (err) { setError(err instanceof Error ? err.message : "Check episode numbers."); return; }
    const current = ++revision.current;
    setBusy(true); setError("");
    try {
      await client.request("/tracker/" + id + "/episodes", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ numbers, ...(progress?.watchCycle ? { expectedCycle: progress.watchCycle } : {}) }) });
      if (!alive.current || current !== revision.current) return;
      await onSaved();
      if (alive.current && current === revision.current) setOpen(false);
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not save episode progress. Try again."); onError(err); }
    } finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  return <div className="mt-4 border-t border-white/10 pt-3">
    {!open ? <button disabled={disabled} aria-label={"Edit watched episodes " + title} onClick={() => void load()} className="text-xs text-violet-200 underline disabled:opacity-50">Edit watched episodes</button> :
      <div role="group" aria-label={"Episode selection " + title}>
        <p className="text-xs text-slate-400">{progress?.mode === "COUNT" ? `Count mode: ${progress.count} watched. Episode identities are unknown; saving replaces this count.` : "Save the exact episodes you have watched, including gaps."}</p>
        <label className="mt-2 block text-xs text-slate-300">Watched episode numbers
          <input aria-label={"Watched episode numbers " + title} value={value} disabled={busy || disabled || !progress} maxLength={70_000} onChange={e => setValue(e.target.value)} placeholder="1, 2, 4" className="mt-1 w-full rounded-lg border border-white/10 bg-slate-950 p-2 text-sm" />
        </label>
        <p className="mt-1 text-xs text-slate-500">Comma-separated numbers. An empty selection clears watched progress.</p>
        {error && <p role="alert" className="mt-2 text-xs text-rose-200">{error}</p>}
        <div className="mt-2 flex gap-3">
          <button disabled={busy || disabled || !progress} onClick={() => void save()} className="text-xs text-violet-200 underline disabled:opacity-50">{busy ? "Please wait…" : "Replace episode selection"}</button>
          <button disabled={busy} onClick={() => setOpen(false)} className="text-xs underline">Cancel episode edit</button>
          {!progress && !busy && <button onClick={() => void load()} className="text-xs underline">Retry episode load</button>}
        </div>
      </div>}
  </div>;
}
