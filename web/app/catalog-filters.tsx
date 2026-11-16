"use client";

import { useEffect, useState } from "react";

export type CatalogFilterValues = {
  mediaType: string; status: string; releaseYear: string; season: string;
  genre: string; theme: string; demographic: string; studio: string;
};
export const emptyCatalogFilters: CatalogFilterValues = {
  mediaType: "", status: "", releaseYear: "", season: "", genre: "", theme: "", demographic: "", studio: "",
};
export default function CatalogFilters({ values, onApply }: {
  values: CatalogFilterValues; onApply: (values: CatalogFilterValues) => void;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(values);
  const [error, setError] = useState("");
  useEffect(() => { setDraft(values); setError(""); }, [values]);
  const applied = Object.values(values).filter(Boolean).length;
  function clear() { setDraft(emptyCatalogFilters); setError(""); onApply(emptyCatalogFilters); }
  return <div className="mt-3">
    <button aria-expanded={open} onClick={() => setOpen(!open)} className="text-sm text-violet-200 underline">{open ? "Hide catalog filters" : "Catalog filters"}{applied ? ` (${applied} applied)` : ""}</button>
    {open && <form aria-label="Catalog metadata filters" className="mt-3 space-y-3 rounded-xl border border-white/10 p-3" onSubmit={event => {
      event.preventDefault();
      const next = { ...draft, genre: draft.genre.trim(), theme: draft.theme.trim(), demographic: draft.demographic.trim(), studio: draft.studio.trim() };
      if ([next.genre, next.theme, next.demographic, next.studio].some(value => new TextEncoder().encode(value).length > 256)) { setError("Each source label must be at most 256 UTF-8 bytes."); return; }
      setError(""); onApply(next);
    }}>
      <p className="text-xs text-slate-400">Filters combine. Source labels match whole values; missing metadata does not match. A release quarter does not confirm episode availability.</p>
      <div className="grid gap-3 sm:grid-cols-2">
        {([
          ["mediaType", "Catalog format", ["TV", "MOVIE", "OVA", "ONA", "SPECIAL", "MUSIC", "UNKNOWN"]],
          ["status", "Catalog release status", ["UNKNOWN", "ANNOUNCED", "RELEASING", "FINISHED", "CANCELLED", "HIATUS"]],
          ["season", "Catalog release quarter", ["WINTER", "SPRING", "SUMMER", "FALL"]],
        ] as const).map(([key, label, options]) => <label key={key} className="text-xs text-slate-300">{label}<select aria-label={label} value={draft[key]} onChange={event => setDraft(current => ({ ...current, [key]: event.target.value }))} className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-900 p-2"><option value="">Any</option>{options.map(value => <option key={value} value={value}>{value}</option>)}</select></label>)}
        <label className="text-xs text-slate-300">Catalog release year<input aria-label="Catalog release year" type="number" min={1900} max={9999} step={1} value={draft.releaseYear} onChange={event => setDraft(current => ({ ...current, releaseYear: event.target.value }))} className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" /></label>
        {([["genre", "Source genre"], ["theme", "Source theme (may contain spoilers)"], ["demographic", "Source demographic"], ["studio", "Source studio"]] as const).map(([key, label]) => <label key={key} className="text-xs text-slate-300">{label}<input aria-label={label} maxLength={256} value={draft[key]} onChange={event => setDraft(current => ({ ...current, [key]: event.target.value }))} className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" /></label>)}
      </div>
      {error && <p role="alert" className="text-xs text-rose-200">{error}</p>}
      <div className="flex gap-4">
        <button type="submit" className="text-xs text-violet-200 underline">Apply catalog filters</button>
        <button type="button" onClick={clear} className="text-xs underline">Clear catalog filters</button>
      </div>
    </form>}
  </div>;
}
