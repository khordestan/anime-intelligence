"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { ApiClient } from "../lib/api";

type Fact = { field: string; values: string[]; provider?: string; observedAt?: string; truncated?: boolean };
type Analysis = { model?: string; topicMatched: boolean; items: { animeId: string; title: string; evidence: Fact[] }[] };

const labels: Record<string, string> = {
  mediaType: "Format",
  releaseYear: "Release year",
  episodeCount: "Episode count",
  status: "Status",
  genres: "Genres",
  studios: "Studios",
};

export default function CatalogAnalysis({ client, animeID, assistedAvailable }: { client: ApiClient; animeID: string; assistedAvailable?: boolean }) {
  const [question, setQuestion] = useState("");
  const [assisted, setAssisted] = useState(false);
  const [result, setResult] = useState<Analysis | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const revision = useRef(0);
  useEffect(() => () => { revision.current++; }, []);

  async function ask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !question.trim()) return;
    const current = ++revision.current;
    setBusy(true);
    setResult(null);
    setFailed(false);
    try {
      const response = await client.request<Analysis>(assistedAvailable && assisted ? "/analysis/catalog/assisted" : "/analysis/catalog", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ question: question.trim(), animeIds: [animeID] }),
      });
      if (current === revision.current) setResult(response);
    } catch {
      if (current === revision.current) setFailed(true);
    } finally {
      if (current === revision.current) setBusy(false);
    }
  }

  return (
    <details className="rounded-lg border border-white/10 p-3 text-xs">
      <summary className="cursor-pointer font-medium">Ask about catalog facts</summary>
      <form onSubmit={ask} className="mt-3 space-y-2">
        <label className="block">
          Question about this title
          <input value={question} maxLength={400} onChange={event => setQuestion(event.target.value)}
            placeholder="How many episodes?" disabled={busy}
            className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
        </label>
        <p className="text-slate-400">Ask about format, release year, status, episode count, genres or studios.</p>
        {assistedAvailable && <label className="block">
          <input type="checkbox" checked={assisted} disabled={busy}
            onChange={event => setAssisted(event.target.checked)} className="mr-2" />
          Let OpenAI select topics using your question
        </label>}
        <button type="submit" disabled={busy || !question.trim()} className="underline disabled:opacity-50">
          {busy ? "Looking up facts…" : "Show evidence"}
        </button>
      </form>
      {failed && <p role="status" className="mt-2 text-rose-200">Could not load catalog evidence. Try again.</p>}
      {result && (
        <section aria-label="Catalog evidence" className="mt-3">
          {result.model && <p className="mb-2">Topics selected by {result.model}</p>}
          {!result.topicMatched && <p className="mb-2">That topic is unavailable. Here are the supported catalog facts.</p>}
          {result.items.map(item => (
            <dl key={item.animeId} className="space-y-2">
              {item.evidence.map(fact => (
                <div key={fact.field}>
                  <dt className="font-medium">{labels[fact.field]}</dt>
                  <dd>{fact.values.join(", ") || "Unknown"}{fact.truncated && " · More values in catalog details"}</dd>
                  {fact.provider && <dd className="text-slate-400">
                    Source: {fact.provider}{fact.observedAt && " · Observed " + new Date(fact.observedAt).toLocaleString()}
                  </dd>}
                </div>
              ))}
            </dl>
          ))}
        </section>
      )}
    </details>
  );
}
