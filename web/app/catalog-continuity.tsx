"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Node = {
  id: string; title: string; mediaType: string; status: string; distance: number;
  episodeCountKnown: boolean; episodeCount?: number; releaseYear?: number; season?: string;
  provider?: string; observedAt?: string;
};
type Graph = {
  rootId: string; nodes: Node[]; edges: { fromId: string; toId: string; providers: string[] }[];
  limit: number; truncated: boolean;
};

export default function CatalogContinuity({ client, id, title, onSelect, onError }: {
  client: ApiClient; id: string; title: string; onSelect: (id: string) => void; onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [graph, setGraph] = useState<Graph | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [edgeLimit, setEdgeLimit] = useState(20);
  const alive = useRef(true);
  const revision = useRef(0);
  useEffect(() => { alive.current = true; return () => { alive.current = false; revision.current++; }; }, []);
  async function load() {
    const current = ++revision.current;
    setOpen(true); setBusy(true); setError(""); setGraph(null); setEdgeLimit(20);
    try {
      const next = await client.request<Graph>("/anime/" + id + "/continuity");
      if (alive.current && current === revision.current) setGraph(next);
    } catch (err) {
      if (alive.current && current === revision.current) { setError("Could not load linked titles. Try again."); onError(err); }
    } finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  const titles = new Map(graph?.nodes.map(node => [node.id, node.title]) ?? []);
  return <div className="pt-2">
    {!open ? <button onClick={() => void load()} className="text-xs text-violet-200 underline">Browse linked titles</button> :
      <section aria-label={"Linked titles for " + title} className="space-y-3 rounded-lg border border-white/10 p-3">
        <div className="flex justify-between gap-3">
          <h5 className="text-xs font-semibold">Linked prequels and sequels</h5>
          <button onClick={() => { revision.current++; setOpen(false); setBusy(false); setGraph(null); }} className="text-xs underline">Close linked titles</button>
        </div>
        <p className="text-xs text-slate-400">Only explicit prequel/sequel links are shown. This may omit franchise titles and is not a recommended watch order.</p>
        {busy && <p role="status" className="text-xs text-slate-400">Loading linked titles…</p>}
        {error && <p role="alert" className="text-xs text-rose-200">{error} <button onClick={() => void load()} className="underline">Retry linked titles</button></p>}
        {graph && <>
          {graph.truncated && <p role="status" className="text-xs text-amber-200">Showing a partial view of up to {graph.limit} titles. Open a linked title to explore from there.</p>}
          {graph.nodes.length === 1 && !graph.truncated && <p className="text-xs text-slate-400">No linked prequels or sequels are available.</p>}
          <ul className="space-y-3">
            {graph.nodes.map(node => <li key={node.id}>
              <button onClick={() => onSelect(node.id)} className="text-xs text-violet-200 underline">View linked title: {node.title}</button>
              <p className="text-xs text-slate-400">{node.mediaType} · {node.status} · {node.episodeCountKnown ? `${node.episodeCount ?? 0} episodes` : "Episode total unknown"}{node.season ? ` · ${node.season}` : ""}{node.releaseYear ? ` ${node.releaseYear}` : ""} · {node.distance === 0 ? "Selected title" : `${node.distance} link${node.distance === 1 ? "" : "s"} away`}</p>
              <p className="text-xs text-slate-500">{node.provider ? `Source: ${node.provider}` : "Source enrichment unavailable"}{node.observedAt ? ` · Observed ${new Date(node.observedAt).toLocaleString()}` : ""}</p>
            </li>)}
          </ul>
          {!!graph.edges.length && <div className="space-y-2">
            <p className="text-xs font-semibold">Reported links (prequel to sequel)</p>
            <ul className="space-y-2">
              {graph.edges.slice(0, edgeLimit).map(edge => <li key={edge.fromId + edge.toId} className="text-xs text-slate-400">{titles.get(edge.fromId) ?? "Unknown title"} → {titles.get(edge.toId) ?? "Unknown title"} · {edge.providers.length ? `Sources: ${edge.providers.join(", ")}` : "Source unavailable"}</li>)}
            </ul>
            {graph.edges.length > edgeLimit && <button onClick={() => setEdgeLimit(edgeLimit + 20)} className="text-xs underline">Show more reported links</button>}
          </div>}
        </>}
      </section>}
  </div>;
}
