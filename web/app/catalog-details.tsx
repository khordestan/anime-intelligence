"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";
import CatalogContinuity from "./catalog-continuity";

type Details = {
  externalIds?: { provider: string; externalId: string }[];
  anime: { id: string; title: string; nativeTitle?: string; synopsis?: string; season?: string; releaseYear?: number; episodeCount?: number; episodeCountKnown: boolean };
  provider?: string; observedAt?: string; genres: string[]; tags: string[]; themes?: string[]; demographics?: string[]; studios: string[]; synonyms: string[];
  relations: { animeId: string; type: string; title?: string; mediaType?: string; status?: string }[];
  episodes: { number: number; track: string; scheduledAt?: string; releasedAt?: string; revision: number }[];
};
export default function CatalogDetails({ client, id, title, onError }: {
  client: ApiClient; id: string; title: string; onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [details, setDetails] = useState<Details | null>(null);
  const [spoilers, setSpoilers] = useState(false);
  const [limit, setLimit] = useState(20);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const alive = useRef(true);
  const revision = useRef(0);
  useEffect(() => { alive.current = true; return () => { alive.current = false; revision.current++; }; }, []);
  async function load(nextId = id) {
    const current = ++revision.current;
    setOpen(true); setBusy(true); setError(""); setDetails(null); setSpoilers(false); setLimit(20);
    try {
      const next = await client.request<Details>("/anime/" + nextId + "/details");
      if (alive.current && current === revision.current) setDetails(next);
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not load catalog details. Try again."); onError(err); }
    } finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  const categorized = new Set([...(details?.themes ?? []), ...(details?.demographics ?? [])].map(value => value.toLowerCase()));
  const otherTags = details?.tags.filter(value => !categorized.has(value.toLowerCase())) ?? [];
  return <div className="mt-3 text-sm">
    {!open ? <button aria-label={"View catalog details " + title} onClick={() => void load()} className="text-xs text-violet-200 underline">View catalog details</button> :
      <section aria-label={"Catalog details " + title} className="rounded-xl border border-white/10 p-3">
        <button onClick={() => { revision.current++; setOpen(false); setBusy(false); }} className="text-xs underline">Close catalog details</button>
        {busy && <p role="status" className="mt-2 text-xs text-slate-400">Loading catalog details…</p>}
        {error && <div role="alert" className="mt-2 text-xs text-rose-200">{error} <button onClick={() => void load()} className="underline">Retry catalog details</button></div>}
        {details && <div className="mt-3 space-y-2">
          <h4 className="font-semibold">{details.anime.title}</h4>
          {details.anime.nativeTitle && <p className="text-slate-400">{details.anime.nativeTitle}</p>}
          <p className="text-xs text-slate-400">{details.anime.season} {details.anime.releaseYear} · {details.anime.episodeCountKnown ? `${details.anime.episodeCount ?? 0} episodes` : "Episode total unknown"}</p>
          <p className="text-xs">Genres: {details.genres.join(", ") || "Not available"}</p>
          <p className="text-xs">Demographics: {details.demographics?.join(", ") || "Not available"}</p>
          <p className="text-xs">Studios: {details.studios.join(", ") || "Not available"}</p>
          {!!details.synonyms.length && <p className="text-xs text-slate-400">Also known as: {details.synonyms.join(", ")}</p>}
          <p className="text-xs text-slate-500">{details.provider ? `Source: ${details.provider}` : "Starter catalog; source enrichment unavailable"}{details.observedAt ? ` · Observed ${new Date(details.observedAt).toLocaleString()}` : ""}</p>
          {!!details.externalIds?.length && <p className="text-xs text-slate-400">Provider IDs: {details.externalIds.map(identity => identity.provider + ": " + identity.externalId).join(" · ")}</p>}
          <button onClick={() => setSpoilers(!spoilers)} className="text-xs text-amber-200 underline">{spoilers ? "Hide synopsis, themes and tags" : "Show synopsis, themes and tags (may contain spoilers)"}</button>
          {spoilers && <div><p className="text-xs text-slate-300">{details.anime.synopsis || "No synopsis available."}</p><p className="mt-2 text-xs text-slate-400">Themes: {details.themes?.join(", ") || "Not available"}</p><p className="mt-2 text-xs text-slate-400">Tags: {otherTags.join(", ") || "Not available"}</p></div>}
          {!!details.relations.length && <div className="flex flex-wrap gap-3">
            {details.relations.map(relation => <button key={relation.type + relation.animeId} disabled={busy} onClick={() => void load(relation.animeId)} className="text-xs text-violet-200 underline">View {relation.type.toLowerCase().replaceAll("_", " ")}{relation.title ? `: ${relation.title}` : " details"}{relation.mediaType ? ` · ${relation.mediaType}` : ""}{relation.status ? ` · ${relation.status}` : ""}</button>)}
          </div>}
          <CatalogContinuity key={details.anime.id} client={client} id={details.anime.id} title={details.anime.title} onSelect={nextId => void load(nextId)} onError={onError} />
          <h5 className="pt-2 text-xs font-semibold">Episode schedule</h5>
          {!details.episodes.length && <p className="text-xs text-slate-400">No episode schedule is available.</p>}
          <ul className="space-y-2">
            {details.episodes.slice(0, limit).map(episode => <li key={episode.number + episode.track} className="text-xs text-slate-400">
              Episode {episode.number} · {episode.track} · {episode.releasedAt ? `Confirmed ${new Date(episode.releasedAt).toLocaleString()}` : episode.scheduledAt ? `Scheduled ${new Date(episode.scheduledAt).toLocaleString()}` : "Time unknown"} · Revision {episode.revision}
            </li>)}
          </ul>
          {details.episodes.length > limit && <button onClick={() => setLimit(limit + 20)} className="text-xs underline">Show more episode schedules</button>}
          <p className="text-xs text-slate-500">Scheduled dates can change. A scheduled date alone does not confirm availability.</p>
        </div>}
      </section>}
  </div>;
}
