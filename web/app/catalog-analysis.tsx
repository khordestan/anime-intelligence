"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { ApiClient } from "../lib/api";
import ProviderCredit from "./provider-credit";

type ComparisonTitle = { id: string; title: string; releaseYear?: number };
type ContinuityFact = { animeId: string; title: string; relation: "PREQUEL" | "SEQUEL"; providers: string[] | null };
type Fact = { field: string; values: string[]; provider?: string; observedAt?: string; truncated?: boolean; episodeFacts?: { number: number; track: string; time: string; provider?: string }[]; continuityFacts?: ContinuityFact[] };
type Comparison = { field: "episodeCount" | "releaseYear"; difference: number; animeIds: [string, string] };
type Analysis = { model?: string; comparisons?: Comparison[]; sharedEvidence?: { field: "genres" | "studios"; values: string[] }[]; topicMatched: boolean; items: { animeId: string; title: string; evidence: Fact[] }[] };

const labels: Record<string, string> = {
  mediaType: "Format",
  releaseYear: "Release year",
  episodeCount: "Episode count",
  status: "Status",
  genres: "Genres",
  studios: "Studios",
  nativeTitle: "Native title",
  season: "Release quarter",
  demographics: "Demographics",
  confirmedReleases: "Confirmed releases",
  upcomingSchedule: "Upcoming schedule (unconfirmed)",
  continuity: "Direct continuity",
};

function explainComparison({ field, difference, animeIds }: Comparison, items: Analysis["items"]) {
  const title = (id: string) => items.find(item => item.animeId === id)?.title;
  const first = title(animeIds[0]), second = title(animeIds[1]);
  if (!first || !second) return null;
  if (difference === 0) return first + " and " + second + (field === "episodeCount" ? " have the same episode count." : " have the same catalog release year.");
  const amount = Math.abs(difference);
  if (field === "episodeCount") {
    return first + " has " + amount + (difference > 0 ? " more" : " fewer") + (amount === 1 ? " episode" : " episodes") + " than " + second + ".";
  }
  return first + " has a catalog release year " + amount + (amount === 1 ? " year" : " years") + (difference < 0 ? " earlier" : " later") + " than " + second + ".";
}

export default function CatalogAnalysis({ client, animeID, assistedAvailable }: { client: ApiClient; animeID: string; assistedAvailable?: boolean }) {
  const [question, setQuestion] = useState("");
  const [episodeNumber, setEpisodeNumber] = useState("");
  const [track, setTrack] = useState("");
  const [comparisonQuery, setComparisonQuery] = useState("");
  const [candidates, setCandidates] = useState<ComparisonTitle[] | null>(null);
  const [comparison, setComparison] = useState<ComparisonTitle | null>(null);
  const [searchBusy, setSearchBusy] = useState(false);
  const [searchFailed, setSearchFailed] = useState(false);
  const [assisted, setAssisted] = useState(false);
  const [result, setResult] = useState<Analysis | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const revision = useRef(0);
  useEffect(() => () => { revision.current++; }, []);

  async function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const query = comparisonQuery.trim();
    if (!query || busy || searchBusy) return;
    const current = ++revision.current;
    setSearchBusy(true);
    setSearchFailed(false);
    try {
      const params = new URLSearchParams({ q: query, limit: "8" });
      const page = await client.request<{ items: ComparisonTitle[] }>("/anime?" + params);
      if (current === revision.current) setCandidates(page.items.filter(item => item.id !== animeID));
    } catch {
      if (current === revision.current) setSearchFailed(true);
    } finally {
      if (current === revision.current) setSearchBusy(false);
    }
  }

  async function ask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || searchBusy || !question.trim()) return;
    const releaseScope = episodeNumber || track ? {
      episodeNumber: episodeNumber ? Number(episodeNumber) : undefined,
      track: track || undefined,
    } : undefined;
    const current = ++revision.current;
    setBusy(true);
    setResult(null);
    setFailed(false);
    try {
      const response = await client.request<Analysis>(assistedAvailable && assisted ? "/analysis/catalog/assisted" : "/analysis/catalog", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ question: question.trim(), animeIds: comparison ? [animeID, comparison.id] : [animeID], releaseScope }),
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
      <details className="mt-3">
        <summary className="cursor-pointer underline">Compare with another title</summary>
        {comparison ? (
          <p className="mt-2">
            Comparing with {comparison.title}{" "}
            <button type="button" disabled={busy} className="underline"
              onClick={() => { setComparison(null); setResult(null); }}>Remove comparison</button>
          </p>
        ) : (
          <>
            <form onSubmit={search} className="mt-2 flex items-end gap-2">
              <label className="grow">
                Find a comparison title
                <input value={comparisonQuery} maxLength={100} disabled={busy || searchBusy}
                  onChange={event => setComparisonQuery(event.target.value)}
                  className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
              </label>
              <button type="submit" disabled={busy || searchBusy || !comparisonQuery.trim()} className="px-2 py-2 underline">
                {searchBusy ? "Searching…" : "Search titles"}
              </button>
            </form>
            {searchFailed && <p role="status" className="mt-2">Could not search the catalog. Try again.</p>}
            {candidates !== null && candidates.length === 0 && <p className="mt-2">No other titles found.</p>}
            {candidates !== null && candidates.length > 0 && <ul className="mt-2 space-y-1">
              {candidates.map(item => <li key={item.id}>
                <button type="button" disabled={busy || searchBusy} className="underline"
                  onClick={() => { setComparison(item); setCandidates(null); setResult(null); }}>
                  {item.title}{item.releaseYear ? " · " + item.releaseYear : ""}
                </button>
              </li>)}
            </ul>}
          </>
        )}
      </details>
      <form onSubmit={ask} className="mt-3 space-y-2">
        <label className="block">
          {comparison ? "Question about these titles" : "Question about this title"}
          <input value={question} maxLength={400} onChange={event => setQuestion(event.target.value)}
            placeholder="How many episodes?" disabled={busy}
            className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
        </label>
        <p className="text-slate-400">Ask about format, release year, status, episode count, genres or studios.</p>
        <details>
          <summary className="cursor-pointer underline">
            Narrow to release evidence{episodeNumber || track ? ` (${[episodeNumber && "episode " + episodeNumber, track].filter(Boolean).join(" · ")})` : ""}
          </summary>
          <div className="mt-2 flex flex-wrap gap-2">
            <label>
              Episode number
              <input disabled={busy} type="number" min={1} max={100000} step={1} value={episodeNumber}
                onChange={event => setEpisodeNumber(event.target.value)}
                className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-2" />
            </label>
            <label>
              Release track
              <select disabled={busy} value={track} onChange={event => setTrack(event.target.value)}
                className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-2">
                <option value="">Any track</option>
                {["SUB", "DUB", "RAW", "UNKNOWN"].map(value => <option key={value} value={value}>{value}</option>)}
              </select>
            </label>
          </div>
        </details>
        {assistedAvailable && <label className="block">
          <input type="checkbox" checked={assisted} disabled={busy}
            onChange={event => setAssisted(event.target.checked)} className="mr-2" />
          Let OpenAI select topics using your question
        </label>}
        <button type="submit" disabled={busy || searchBusy || !question.trim()} className="underline disabled:opacity-50">
          {busy ? "Looking up facts…" : "Show evidence"}
        </button>
      </form>
      {failed && <p role="status" className="mt-2 text-rose-200">Could not load catalog evidence. Try again.</p>}
      {result && (
        <section aria-label="Catalog evidence" className="mt-3">
          {result.model && <p className="mb-2">Topics selected by {result.model}</p>}
          {!result.topicMatched && <p className="mb-2">That topic is unavailable. Here are the supported catalog facts.</p>}
          {!!result.comparisons?.length && <ul className="mb-3 space-y-1">
            {result.comparisons.map(comparison => <li key={comparison.field}>{explainComparison(comparison, result.items)}</li>)}
          </ul>}
          {!!result.sharedEvidence?.length && <dl className="mb-3 space-y-1">
            {result.sharedEvidence.map(({ field, values }) => (
              <div key={field}>
                <dt className="font-medium">{field === "genres" ? "Shared genres" : "Shared studios"}</dt>
                <dd>{values.join(", ")}</dd>
              </div>
            ))}
          </dl>}
          <div className={result.items.length > 1 ? "grid gap-3 sm:grid-cols-2" : undefined}>
            {result.items.map(item => (
              <div key={item.animeId}>
                {result.items.length > 1 && <h5 className="mb-2 font-semibold">{item.title}</h5>}
                <dl className="space-y-2">
                  {item.evidence.map(fact => (
                    <div key={fact.field}>
                      <dt className="font-medium">{labels[fact.field]}</dt>
                      <dd>
                        {fact.episodeFacts?.length ? (
                          <ul className="space-y-1">
                            {fact.episodeFacts.map(episode => (
                              <li key={episode.number + ":" + episode.track}>
                                Episode {episode.number} · {episode.track} · <time dateTime={episode.time}>{new Date(episode.time).toLocaleString()}</time>
                                {episode.provider && <> · {episode.provider === "tvmaze" ? <ProviderCredit provider="tvmaze" /> : "Source: " + episode.provider}</>}
                              </li>
                            ))}
                          </ul>
                        ) : fact.continuityFacts?.length ? (
                          <ul className="space-y-1">
                            {fact.continuityFacts.map(link => (
                              <li key={link.relation + link.animeId}>
                                {link.relation === "PREQUEL" ? "Prequel" : "Sequel"} · {link.title}
                                {link.providers?.map(provider => <span key={provider}> · {provider === "tvmaze" ? <ProviderCredit provider="tvmaze" /> : "Source: " + provider}</span>)}
                              </li>
                            ))}
                          </ul>
                        ) : fact.values.join(", ") || "Unknown"}
                        {fact.truncated && <span> · {fact.field === "continuity" ? "Continuity evidence is limited" : "More values in catalog details"}</span>}
                      </dd>
                      {fact.provider && <dd className="text-slate-400">
                        Source: {fact.provider}{fact.observedAt && " · Observed " + new Date(fact.observedAt).toLocaleString()}
                      </dd>}
                    </div>
                  ))}
                </dl>
              </div>
            ))}
          </div>
        </section>
      )}
    </details>
  );
}
