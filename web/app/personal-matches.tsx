"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Preference = { key: string; name: string; category: string; weight: number | null };
type Match = { animeId: string; title: string; score: number; evidence: { featureKey: string; direction: string; strength: number; impact: number }[] };
export default function PersonalMatches({ client, onError }: { client: ApiClient; onError: (error: unknown) => void }) {
  const [open, setOpen] = useState(false);
  const [preferences, setPreferences] = useState<Preference[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [matches, setMatches] = useState<Match[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const alive = useRef(true), revision = useRef(0);
  useEffect(() => { alive.current = true; return () => { alive.current = false; revision.current++; }; }, []);
  async function read(current: number) {
    const [p, m] = await Promise.all([
      client.request<{ items: Preference[] }>("/preferences"),
      client.request<{ items: Match[] }>("/recommendations"),
    ]);
    if (!alive.current || current !== revision.current) return;
    setPreferences(p.items); setMatches(m.items);
  }
  async function load() {
    const current = ++revision.current;
    setOpen(true); setBusy(true); setMessage("");
    try { await read(current); }
    catch (error) { if (alive.current && current === revision.current) { setMessage("Could not load Personal Match. Try again."); onError(error); } }
    finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  async function save(preference: Preference, reset = false) {
    const value = drafts[preference.key] ?? (preference.weight === null ? "" : String(preference.weight));
    const weight = Number(value);
    if (!reset && (!value.trim() || !Number.isFinite(weight) || weight < -1 || weight > 1)) { setMessage("Use a preference weight between -1 and 1."); return; }
    const current = ++revision.current;
    setBusy(true); setMessage("");
    try {
      await client.request("/preferences/" + encodeURIComponent(preference.key), {
        method: reset ? "DELETE" : "PUT",
        headers: { "Content-Type": "application/json" },
        body: reset ? undefined : JSON.stringify({ weight }),
      });
      if (!alive.current || current !== revision.current) return;
      await read(current);
      if (alive.current && current === revision.current) {
        setDrafts(existing => { const next = { ...existing }; delete next[preference.key]; return next; });
        setMessage("Preference updated.");
      }
    } catch (error) { if (alive.current && current === revision.current) { setMessage("Could not save preference. Try again."); onError(error); } }
    finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  const names = new Map(preferences.map(p => [p.key, p.name]));
  return <section aria-label="Personal Match" className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5">
    <button disabled={busy} aria-expanded={open} onClick={() => { if (open) setOpen(false); else void load(); }} className="text-sm text-violet-200 underline disabled:opacity-50">
      {open ? "Hide Personal Match" : "Personal Match settings"}
    </button>
    {open && <div className="mt-4">
      <h2 className="font-semibold">Your feature preferences</h2>
      <p className="mt-2 text-sm text-slate-400">Use -1 to avoid a feature, 0 for neutral and +1 to prefer it. Reset removes only your declared preference; your ratings can still contribute when evidence supports them.</p>
      <button disabled={busy} onClick={() => void load()} className="mt-3 text-xs underline disabled:opacity-50">Refresh Personal Match</button>
      <p role="status" className="mt-2 text-sm text-slate-400">{busy ? "Loading Personal Match…" : message}</p>
      <ul className="mt-3 grid gap-3 sm:grid-cols-2" aria-label="Feature preferences">
        {preferences.map(p => <li key={p.key} className="rounded-xl bg-white/5 p-3">
          <label className="text-sm">{p.name}
            <input type="number" min={-1} max={1} step="any" aria-label={"Preference " + p.name} placeholder="Not set" disabled={busy}
              value={drafts[p.key] ?? (p.weight === null ? "" : String(p.weight))}
              onChange={event => setDrafts(current => ({ ...current, [p.key]: event.target.value }))}
              className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
          </label>
          <p className="mt-1 text-xs text-slate-500">{p.category}</p>
          <div className="mt-2 flex gap-3">
            <button disabled={busy} aria-label={"Save preference " + p.name} onClick={() => void save(p)} className="text-xs text-violet-200 underline disabled:opacity-50">Save preference</button>
            <button disabled={busy} aria-label={"Reset preference " + p.name} onClick={() => void save(p, true)} className="text-xs underline disabled:opacity-50">Reset preference</button>
          </div>
        </li>)}
      </ul>
      <h3 className="mt-5 font-semibold">Personal matches</h3>
      {!busy && matches.length === 0 && <p className="mt-2 text-sm text-slate-400">No supported matches yet. Curated catalog evidence and your preferences or ratings are needed.</p>}
      <ul className="mt-3 space-y-3" aria-label="Personal match results">
        {matches.map(match => <li key={match.animeId} className="rounded-xl bg-white/5 p-3">
          <p className="text-sm font-medium">{match.title} <span className="text-violet-200">{match.score.toFixed(2)} / 100</span></p>
          <ul className="mt-2 space-y-1">{match.evidence.map(e => <li key={e.featureKey} className="text-xs text-slate-400">{names.get(e.featureKey) ?? e.featureKey}: {e.direction}</li>)}</ul>
        </li>)}
      </ul>
      <p className="mt-4 text-xs text-slate-500">Compatibility scores are not probabilities. Matches consider up to 500 untracked titles and 1,000 recent series ratings. Unsupported or spoiler-sensitive observations are excluded.</p>
    </div>}
  </section>;
}
