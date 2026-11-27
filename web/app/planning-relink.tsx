"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { ApiClient, ApiError } from "../lib/api";

type Candidate = { id: string; title: string; releaseYear?: number; mediaType?: string };

export default function PlanningRelink({ client, entryId, animeId, title, updatedAt, disabled, onSaved, onError }: {
  client: ApiClient;
  entryId: string;
  animeId: string;
  title: string;
  updatedAt: string;
  disabled: boolean;
  onSaved: () => Promise<void>;
  onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [candidates, setCandidates] = useState<Candidate[] | null>(null);
  const [review, setReview] = useState<{ title: Candidate; expectedUpdatedAt: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const revision = useRef(0);
  useEffect(() => () => { revision.current++; }, []);

  async function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!query.trim() || busy || disabled) return;
    const current = ++revision.current;
    setBusy(true);
    setError("");
    setReview(null);
    setCandidates(null);
    try {
      const params = new URLSearchParams({ q: query.trim(), limit: "8" });
      const result = await client.request<{ items: Candidate[] }>("/anime?" + params);
      if (current === revision.current) setCandidates(result.items.filter(item => item.id !== animeId));
    } catch (failure) {
      if (current === revision.current) {
        setError("Could not search the catalog. Try again.");
        onError(failure);
      }
    } finally {
      if (current === revision.current) setBusy(false);
    }
  }

  async function save() {
    if (!review || busy || disabled) return;
    const current = ++revision.current;
    setBusy(true);
    setError("");
    try {
      await client.request("/tracker/" + entryId + "/relink-planning", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ targetAnimeId: review.title.id, expectedUpdatedAt: review.expectedUpdatedAt }),
      });
      if (current === revision.current) {
        await onSaved();
        if (current === revision.current) setOpen(false);
      }
    } catch (failure) {
      if (current === revision.current) {
        setError(failure instanceof ApiError && (failure.status === 409 || failure.status === 404)
          ? "The entry or destination changed. Refresh the library and review again."
          : "Could not correct the planning title. Try again.");
        onError(failure);
      }
    } finally {
      if (current === revision.current) setBusy(false);
    }
  }

  return (
    <div className="mt-4 border-t border-white/10 pt-3 text-xs">
      {open ? (
        <section aria-label={"Correct planning title for " + title}>
          {!review && (
            <form onSubmit={search} className="flex items-end gap-2">
              <label className="grow">
                Find the intended title
                <input value={query} maxLength={100} disabled={busy || disabled}
                  onChange={event => setQuery(event.target.value)}
                  className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
              </label>
              <button type="submit" disabled={busy || disabled || !query.trim()} className="p-2 underline">Search titles</button>
            </form>
          )}
          {!review && candidates && (candidates.length ? (
            <ul className="mt-2 space-y-1">
              {candidates.map(candidate => (
                <li key={candidate.id}>
                  <button type="button" disabled={busy || disabled} onClick={() => setReview({ title: candidate, expectedUpdatedAt: updatedAt })} className="underline">
                    {candidate.title}{candidate.releaseYear ? " · " + candidate.releaseYear : ""}{candidate.mediaType ? " · " + candidate.mediaType : ""}
                  </button>
                </li>
              ))}
            </ul>
          ) : <p className="mt-2 text-slate-300">No other title matches.</p>)}
          {review && (
            <div className="mt-2 text-slate-300">
              <p>Replace the empty Planning entry for <strong>{title}</strong> with a new Planning entry for <strong>{review.title.title}</strong>?</p>
              <button type="button" disabled={busy || disabled} onClick={() => void save()} className="mt-2 text-violet-200 underline disabled:opacity-50">Confirm planning title correction</button>
            </div>
          )}
          {error && <p role="alert" className="mt-2 text-rose-200">{error}</p>}
          <button type="button" disabled={busy} onClick={() => {
            setOpen(false);
            setQuery("");
            setReview(null);
            setCandidates(null);
            setError("");
          }} className="mt-2 underline">Cancel correction</button>
        </section>
      ) : <button type="button" disabled={busy || disabled} aria-label={"Correct planning title for " + title}
        onClick={() => setOpen(true)} className="text-violet-200 underline disabled:opacity-50">Correct planning title</button>}
    </div>
  );
}
