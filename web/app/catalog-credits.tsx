"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

export type Credit = { name: string; role: string };

export function CreditList({ items }: { items: Credit[] }) {
  return <ul className="mt-1 space-y-1 text-xs text-slate-300">
    {items.map(({ name, role }) => <li key={JSON.stringify([name, role])}>{name} · {role}</li>)}
  </ul>;
}

type Credits = { items: Credit[]; provider?: string };

export default function CharacterCredits({ client, animeId }: { client: ApiClient; animeId: string }) {
  const [open, setOpen] = useState(false);
  const [credits, setCredits] = useState<Credits | null>(null);
  const [failed, setFailed] = useState(false);
  const revision = useRef(0);
  useEffect(() => () => { revision.current++; }, []);

  async function load() {
    const current = ++revision.current;
    setOpen(true);
    setFailed(false);
    setCredits(null);
    try {
      const result = await client.request<Credits>("/anime/" + animeId + "/character-credits");
      if (current === revision.current) setCredits(result);
    } catch {
      if (current === revision.current) setFailed(true);
    }
  }

  return <div className="pt-2">
    <button type="button" aria-expanded={open} className="text-xs text-amber-200 underline" onClick={() => {
      if (open) {
        revision.current++;
        setOpen(false);
        setCredits(null);
      } else void load();
    }}>
      {open ? "Hide character credits" : "Show character credits (may contain spoilers)"}
    </button>
    {open && <section aria-label="Character credits" className="mt-1 text-xs">
      {failed ? <button type="button" onClick={() => void load()} className="underline">Retry character credits</button> :
        credits ? <>
          {credits.items.length > 0 ? <CreditList items={credits.items} /> : <p>No character credits are available.</p>}
          {credits.provider && <p className="mt-1 text-slate-400">Credits source: {credits.provider}</p>}
        </> : <p role="status">Loading character credits…</p>}
    </section>}
  </div>;
}
