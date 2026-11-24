"use client";

import { useMemo, useState } from "react";
import { ApiClient } from "../lib/api";

type Evidence = {
  kind: "JURIED_AWARD_WIN" | "INSTITUTIONAL_SELECTION";
  publisherId: string;
  sourceUrl: string;
};

type EssentialsTitle = { animeId: string; title: string; evidence: Evidence[] };

const label: Record<Evidence["kind"], string> = {
  JURIED_AWARD_WIN: "Juried award win",
  INSTITUTIONAL_SELECTION: "Preservation selection",
};

export default function GlobalEssentials({ baseURL }: { baseURL: string }) {
  const client = useMemo(() => new ApiClient(baseURL), [baseURL]);
  const [items, setItems] = useState<EssentialsTitle[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);

  async function load() {
    setBusy(true);
    setFailed(false);
    try {
      const page = await client.request<{ items: EssentialsTitle[] }>("/recommendations/essentials");
      setItems(page.items);
    } catch {
      setFailed(true);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section aria-labelledby="global-essentials-title" className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5">
      <h2 id="global-essentials-title" className="font-semibold">Global Essentials</h2>
      {items === null && (
        <button type="button" disabled={busy} onClick={load} className="mt-3 rounded-lg border border-white/20 px-3 py-2 text-sm">
          {busy ? "Loading…" : failed ? "Retry Global Essentials" : "Browse Global Essentials"}
        </button>
      )}
      {failed && <p role="status" className="mt-2 text-sm">Could not load Global Essentials.</p>}
      {items !== null && (
        <ol className="mt-3 list-decimal space-y-3 pl-6">
          {items.map((item) => (
            <li key={item.animeId}>
              <h3 className="font-medium">{item.title}</h3>
              <ul className="mt-1 space-y-1 text-sm">
                {item.evidence.map((evidence) => (
                  <li key={JSON.stringify(evidence)}>
                    <a href={evidence.sourceUrl} className="underline">
                      {label[evidence.kind]} · {evidence.publisherId}
                    </a>
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
