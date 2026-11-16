"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Notes = { notes: string; updatedAt: string };
export default function NotesEditor({ client, id, title, disabled, onSaved, onError }: {
  client: ApiClient; id: string; title: string; disabled: boolean;
  onSaved: () => Promise<void>; onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false), [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState<Notes | null>(null);
  const [draft, setDraft] = useState(""), [error, setError] = useState("");
  const alive = useRef(true), revision = useRef(0), inFlight = useRef(false);
  useEffect(() => { alive.current = true; return () => { alive.current = false; revision.current++; }; }, []);
  async function load() {
    if (inFlight.current) return;
    inFlight.current = true;
    const current = ++revision.current;
    setOpen(true); setBusy(true); setError("");
    try {
      const notes = await client.request<Notes>("/tracker/" + id + "/notes");
      if (alive.current && current === revision.current) { setSaved(notes); setDraft(notes.notes); }
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not load private notes. Try again."); onError(err); }
    } finally { if (alive.current && current === revision.current) { setBusy(false); inFlight.current = false; } }
  }
  async function save() {
    if (inFlight.current || !saved) return;
    if (Array.from(draft).length > 2000) { setError("Use at most 2,000 characters."); return; }
    inFlight.current = true;
    const current = ++revision.current;
    setBusy(true); setError("");
    try {
      await client.request("/tracker/" + id + "/notes", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ notes: draft, expectedUpdatedAt: saved.updatedAt }) });
      if (!alive.current || current !== revision.current) return;
      await onSaved();
      if (alive.current && current === revision.current) { setOpen(false); setDraft(""); setSaved(null); }
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not save notes. Your draft is kept. If this entry changed elsewhere, reload before saving."); onError(err); }
    } finally { if (alive.current && current === revision.current) { setBusy(false); inFlight.current = false; } }
  }
  return <div className="mt-4 border-t border-white/10 pt-3">
    {!open ? <button disabled={disabled} aria-label={"Edit private notes " + title} onClick={() => void load()} className="text-xs text-violet-200 underline disabled:opacity-50">Private notes</button> :
      <div role="group" aria-label={"Private notes " + title}>
        <label className="block text-xs text-slate-300">Your private notes
          <textarea aria-label={"Notes " + title} value={draft} maxLength={4000} disabled={busy || disabled || !saved} onChange={e => setDraft(e.target.value)} className="mt-2 block w-full rounded-lg border border-white/10 bg-slate-950 p-2 text-sm" />
        </label>
        <p className="mt-1 text-xs text-slate-500">{Array.from(draft).length} / 2,000 characters. Plain text, saved only when you choose Save notes. An empty note clears it.</p>
        {error && <p role="alert" className="mt-2 text-xs text-rose-200">{error}</p>}
        <div className="mt-3 flex flex-wrap gap-3">
          <button disabled={busy || disabled || !saved} onClick={() => void save()} className="text-xs text-violet-200 underline disabled:opacity-50">Save notes</button>
          <button disabled={busy || disabled} onClick={() => void load()} className="text-xs underline disabled:opacity-50">{saved ? "Discard draft and reload notes" : "Retry notes load"}</button>
          <button disabled={busy} onClick={() => { revision.current++; setOpen(false); setDraft(""); setSaved(null); }} className="text-xs underline">Cancel notes edit</button>
        </div>
      </div>}
  </div>;
}
