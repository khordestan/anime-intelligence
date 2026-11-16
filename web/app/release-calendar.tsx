"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Calendar = { from: string; to: string; limit: number; truncated: boolean; items: { animeId: string; title: string; number: number; track: string; scheduledAt: string; verifiedAt?: string; revision: number; provider?: string }[] };
export default function ReleaseCalendar({ client, onError }: { client: ApiClient; onError: (error: unknown) => void }) {
  const [open, setOpen] = useState(false), [busy, setBusy] = useState(false);
  const [from, setFrom] = useState(""), [days, setDays] = useState("14"), [error, setError] = useState("");
  const [calendar, setCalendar] = useState<Calendar | null>(null);
  const alive = useRef(true), revision = useRef(0), inFlight = useRef(false);
  useEffect(() => { alive.current = true; setFrom(new Date().toISOString().slice(0,10)); return () => { alive.current = false; revision.current++; }; }, []);
  async function load() {
    if (inFlight.current) return;
    const start = from || new Date().toISOString().slice(0,10);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(start)) { setError("Choose a start date."); return; }
    inFlight.current = true;
    const current = ++revision.current;
    setFrom(start); setOpen(true); setBusy(true); setError("");
    try {
      const next = await client.request<Calendar>("/calendar?from=" + encodeURIComponent(start) + "&days=" + days);
      if (alive.current && current === revision.current) setCalendar(next);
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not load followed schedules. Try again."); onError(err); }
    } finally { if (alive.current && current === revision.current) { setBusy(false); inFlight.current = false; } }
  }
  return <section aria-label="Followed release calendar" className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5">
    <button disabled={busy} aria-expanded={open} onClick={() => { if (open) { revision.current++; setOpen(false); } else void load(); }} className="text-sm text-violet-200 underline disabled:opacity-50">{open ? "Hide followed schedules" : "Upcoming followed schedules"}</button>
    {open && <div className="mt-4">
      <h2 className="font-semibold">Followed release calendar</h2>
      <p className="mt-2 text-sm text-slate-400">Unconfirmed schedules can change. Confirmed-release notices appear in Release alerts. Each follow's track and season scope applies here.</p>
      <div className="mt-3 flex flex-wrap items-end gap-3">
        <label className="text-xs text-slate-400">Start date (UTC)<input aria-label="Calendar start date (UTC)" type="date" value={from} disabled={busy} onChange={e => setFrom(e.target.value)} className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-2" /></label>
        <label className="text-xs text-slate-400">Days<select aria-label="Calendar days" value={days} disabled={busy} onChange={e => setDays(e.target.value)} className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-2">{[1,7,14,31].map(n => <option key={n} value={n}>{n}</option>)}</select></label>
        <button disabled={busy} onClick={() => void load()} className="text-xs underline disabled:opacity-50">Refresh followed schedules</button>
      </div>
      {busy && <p role="status" className="mt-3 text-sm text-slate-400">Loading followed schedules…</p>}
      {error && <p role="alert" className="mt-3 text-sm text-rose-200">{error}</p>}
      {calendar && <div className="mt-3">
        <p className="text-xs text-slate-500">Window: {calendar.from.slice(0,10)} to {calendar.to.slice(0,10)} (end exclusive, UTC)</p>
        {!calendar.items.length && <p className="mt-2 text-sm text-slate-400">No unconfirmed schedules in this window for your followed tracks. Add or change follows in Release alerts.</p>}
        <ul aria-label="Followed schedules" className="mt-3 space-y-3">
          {calendar.items.map(item => <li key={item.animeId + ":" + item.number + ":" + item.track} className="rounded-xl bg-white/5 p-3">
            <p className="text-sm font-medium">{item.title} · Episode {item.number} · {item.track}</p>
            <p className="mt-1 text-xs text-slate-400">Scheduled {new Date(item.scheduledAt).toLocaleString()} · Unconfirmed · Revision {item.revision}</p>
            <p className="mt-1 text-xs text-slate-500">{item.provider ? "Source: " + item.provider : "Source unavailable"}{item.verifiedAt ? " · Observed " + new Date(item.verifiedAt).toLocaleString() : ""}</p>
          </li>)}
        </ul>
        {calendar.truncated && <p className="mt-3 text-xs text-amber-200">Showing the first {calendar.limit} schedules. Choose fewer days to narrow the window.</p>}
      </div>}
    </div>}
  </section>;
}
