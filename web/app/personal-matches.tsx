"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Preference = { key: string; name: string; category: string; weight: number | null };
type Signal = { key: string; declared?: { weight: number; confidence: number }; inferred?: { weight: number; confidence: number; ratingCount?: number } };
type HiddenMatch = { animeId: string; title: string };
type DiscoveryTitle = HiddenMatch & { evidenceCount: number; supportedFeatures?: string[]; unfamiliarCount?: number; featuredEvidence?: { key: string; name: string }[] };
type DiscoveryCursor = Pick<DiscoveryTitle, "animeId" | "evidenceCount"> & { featureKey?: string; unfamiliar?: boolean };
type DiscoveryPage = { items: DiscoveryTitle[]; nextCursor?: DiscoveryCursor };
type HiddenMatches = { items: HiddenMatch[]; nextCursor?: string };
type Match = { animeId: string; title: string; score: number; evidence: { featureKey: string; direction: string; strength: number; impact: number }[] };
export default function PersonalMatches({ client, onError, onLibraryChange }: { client: ApiClient; onError: (error: unknown) => void; onLibraryChange: () => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const [preferences, setPreferences] = useState<Preference[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [lastDismissed, setLastDismissed] = useState<HiddenMatch | null>(null);
  const [discovery, setDiscovery] = useState<DiscoveryPage | null>(null);
  const [discoveryFeature, setDiscoveryFeature] = useState("");
  const [discoveryUnfamiliar, setDiscoveryUnfamiliar] = useState(false);
  const [hidden, setHidden] = useState<HiddenMatches | null>(null);
  const [matches, setMatches] = useState<Match[]>([]);
  const [profile, setProfile] = useState<Signal[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const alive = useRef(true), revision = useRef(0);
  useEffect(() => { alive.current = true; return () => { alive.current = false; revision.current++; }; }, []);
  async function read(current: number, includeHidden = hidden !== null, includeDiscovery = discovery !== null) {
    const [p, m, h, d] = await Promise.all([
      client.request<{ items: Preference[] }>("/preferences"),
      client.request<{ items: Match[]; profile?: Signal[] }>("/recommendations"),
      includeHidden ? client.request<HiddenMatches>("/recommendations/dismissals") : Promise.resolve(null),
      includeDiscovery ? client.request<DiscoveryPage>(discoveryPath(discoveryFeature)) : Promise.resolve(null),
    ]);
    if (!alive.current || current !== revision.current) return;
    setHidden(h);
    setDiscovery(d);
    setPreferences(p.items); setMatches(m.items); setProfile(m.profile ?? []);
  }
  async function load(includeHidden = hidden !== null, includeDiscovery = discovery !== null) {
    const current = ++revision.current;
    setOpen(true); setBusy(true); setMessage("");
    try { await read(current, includeHidden, includeDiscovery); }
    catch (error) { if (alive.current && current === revision.current) { setMessage("Could not load Personal Match. Try again."); onError(error); } }
    finally { if (alive.current && current === revision.current) setBusy(false); }
  }
  async function save(preference: Preference, reset = false, discoveryWeight?: number) {
    const value = discoveryWeight === undefined
      ? drafts[preference.key] ?? (preference.weight === null ? "" : String(preference.weight))
      : String(discoveryWeight);
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
  async function setDismissed(match: HiddenMatch, dismissed: boolean) {
    const current = ++revision.current;
    setBusy(true); setMessage("");
    try {
      await client.request("/recommendations/dismissals/" + encodeURIComponent(match.animeId), {
        method: dismissed ? "PUT" : "DELETE",
      });
      if (!alive.current || current !== revision.current) return;
      setLastDismissed(dismissed ? match : null);
      if (dismissed) {
        setMatches(currentMatches => currentMatches.filter(item => item.animeId !== match.animeId));
        setDiscovery(page => page ? { ...page, items: page.items.filter(item => item.animeId !== match.animeId) } : null);
      }
      await read(current);
      if (alive.current && current === revision.current) setMessage(dismissed ? "Match hidden." : "Match restored.");
    } catch (error) {
      if (alive.current && current === revision.current) {
        setMessage("Could not finish updating Personal Match. Try refreshing.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) setBusy(false);
    }
  }
  async function planTitle(title: HiddenMatch) {
    const current = ++revision.current;
    setBusy(true);
    setMessage("");
    try {
      const result = await client.request<{ added: number }>("/tracker/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ version: 1, entries: [{ animeId: title.animeId, watchStatus: "PLANNING" }] }),
      });
      if (!alive.current || current !== revision.current) return;
      setMatches(items => items.filter(item => item.animeId !== title.animeId));
      setDiscovery(page => page ? { ...page, items: page.items.filter(item => item.animeId !== title.animeId) } : null);
      await Promise.all([read(current), onLibraryChange()]);
      if (alive.current && current === revision.current) {
        setMessage(result.added ? "Added to your library as Planning." : "Already in your library.");
      }
    } catch (error) {
      if (alive.current && current === revision.current) {
        setMessage("Could not finish adding this title. Try refreshing.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) setBusy(false);
    }
  }

  async function loadMoreHidden(after: string) {
    const current = ++revision.current;
    setBusy(true);
    setMessage("");
    try {
      const page = await client.request<HiddenMatches>("/recommendations/dismissals?after=" + encodeURIComponent(after));
      if (alive.current && current === revision.current) {
        setHidden(existing => existing ? { ...page, items: [...existing.items, ...page.items] } : page);
      }
    } catch (error) {
      if (alive.current && current === revision.current) {
        setMessage("Could not load hidden matches. Try again.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) setBusy(false);
    }
  }
  function discoveryPath(featureKey: string, after?: DiscoveryCursor, unfamiliar = discoveryUnfamiliar) {
    const query = new URLSearchParams();
    if (featureKey) query.set("featureKey", featureKey);
    if (unfamiliar) query.set("unfamiliar", "true");
    if (after) {
      query.set("after", after.animeId);
      query.set("afterCount", String(after.evidenceCount));
      if (after.featureKey) query.set("afterFeature", after.featureKey);
      if (after.unfamiliar) query.set("afterUnfamiliar", "true");
    }
    return "/recommendations/discovery" + (query.size ? "?" + query : "");
  }

  async function filterDiscovery(featureKey: string, unfamiliar = discoveryUnfamiliar) {
    const current = ++revision.current;
    setBusy(true);
    setMessage("");
    try {
      const page = await client.request<DiscoveryPage>(discoveryPath(featureKey, undefined, unfamiliar));
      if (alive.current && current === revision.current) {
        setDiscoveryFeature(featureKey);
        setDiscoveryUnfamiliar(unfamiliar);
        setDiscovery(page);
      }
    } catch (error) {
      if (alive.current && current === revision.current) {
        setMessage("Could not filter Discovery titles. Try again.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) setBusy(false);
    }
  }

  async function loadMoreDiscovery(after: DiscoveryCursor) {
    const current = ++revision.current;
    setBusy(true);
    setMessage("");
    try {
      const page = await client.request<DiscoveryPage>(discoveryPath(discoveryFeature, after));
      if (alive.current && current === revision.current) {
        setDiscovery(existing => {
          const previous = existing?.items ?? [];
          const seen = new Set(previous.map(item => item.animeId));
          return { ...page, items: [...previous, ...page.items.filter(item => !seen.has(item.animeId))] };
        });
      }
    } catch (error) {
      if (alive.current && current === revision.current) {
        setMessage("Could not load more discovery titles. Try again.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) setBusy(false);
    }
  }
  const names = new Map(preferences.map(p => [p.key, p.name]));
  const selectedFeature = preferences.find(preference => preference.key === discoveryFeature);
  const inferred = new Map(profile.filter(p => p.inferred).map(p => [p.key, p.inferred!]));
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
          {inferred.has(p.key) && <p className="mt-1 text-xs text-slate-400">Rating-derived signal: {inferred.get(p.key)!.weight > 0 ? "prefers" : inferred.get(p.key)!.weight < 0 ? "avoids" : "neutral toward"} this feature · strength {inferred.get(p.key)!.confidence.toFixed(2)}{inferred.get(p.key)!.ratingCount != null && <span> · {inferred.get(p.key)!.ratingCount} supporting ratings</span>}</p>}
          <div className="mt-2 flex gap-3">
            <button disabled={busy} aria-label={"Save preference " + p.name} onClick={() => void save(p)} className="text-xs text-violet-200 underline disabled:opacity-50">Save preference</button>
            <button disabled={busy} aria-label={"Reset preference " + p.name} onClick={() => void save(p, true)} className="text-xs underline disabled:opacity-50">Reset preference</button>
          </div>
        </li>)}
      </ul>
      <h3 className="mt-5 font-semibold">Personal matches</h3>
      <button type="button" disabled={busy} aria-expanded={hidden !== null}
        onClick={() => { if (hidden) setHidden(null); else void load(true); }}
        className="mt-2 mr-4 text-xs underline disabled:opacity-50">{hidden ? "Close hidden matches" : "Manage hidden matches"}</button>
      {lastDismissed && <button type="button" disabled={busy} onClick={() => void setDismissed(lastDismissed, false)} className="mt-2 text-xs underline disabled:opacity-50">Undo hiding {lastDismissed.title}</button>}
      {hidden && (
        <section aria-label="Hidden Personal Matches" className="mt-3 rounded-xl bg-white/5 p-3">
          {hidden.items.length === 0 && <p className="text-sm text-slate-400">No hidden matches.</p>}
          <ul className="space-y-2">
            {hidden.items.map(match => (
              <li key={match.animeId} className="flex items-center justify-between gap-3 text-sm">
                <span>{match.title}</span>
                <button type="button" disabled={busy} aria-label={"Restore match " + match.title}
                  onClick={() => void setDismissed(match, false)} className="text-xs underline disabled:opacity-50">Restore</button>
              </li>
            ))}
          </ul>
          {hidden.nextCursor && <button type="button" disabled={busy} onClick={() => void loadMoreHidden(hidden.nextCursor!)}
            className="mt-2 text-xs underline disabled:opacity-50">Load more hidden matches</button>}
        </section>
      )}
      {!busy && matches.length === 0 && <p className="mt-2 text-sm text-slate-400">No supported matches yet. Curated catalog evidence and your preferences or ratings are needed.</p>}
      <ul className="mt-3 space-y-3" aria-label="Personal match results">
        {matches.map(match => <li key={match.animeId} className="rounded-xl bg-white/5 p-3">
          <p className="text-sm font-medium">{match.title} <span className="text-violet-200">{match.score.toFixed(2)} / 100</span></p>
          <button type="button" disabled={busy} aria-label={"Plan to watch " + match.title}
            onClick={() => void planTitle(match)} className="mr-3 mt-2 text-xs underline disabled:opacity-50">Plan to watch</button>
          <button type="button" disabled={busy} aria-label={"Hide match " + match.title} onClick={() => void setDismissed(match, true)} className="mt-2 text-xs underline disabled:opacity-50">Hide match</button>
          <ul className="mt-2 space-y-1">{match.evidence.map(e => <li key={e.featureKey} className="text-xs text-slate-400">{names.get(e.featureKey) ?? e.featureKey}: {e.direction}</li>)}</ul>
        </li>)}
      </ul>
      <div className="mt-5 border-t border-white/10 pt-4">
        <h3 className="font-semibold">Discovery</h3>
        <button type="button" disabled={busy} aria-expanded={discovery !== null}
          onClick={() => { if (discovery !== null) setDiscovery(null); else void load(hidden !== null, true); }}
          className="mt-2 text-xs underline disabled:opacity-50">{discovery !== null ? "Close Discovery" : "Explore titles"}</button>
        {discovery !== null && (
          <section aria-label="Discovery titles" className="mt-3">
            <label className="mt-2 flex items-center gap-2 text-sm">
              <input type="checkbox" checked={discoveryUnfamiliar} disabled={busy}
                onChange={event => void filterDiscovery(discoveryFeature, event.target.checked)} />
              Explore features missing from my library
            </label>
            <p className="mt-1 text-sm text-slate-400">
              {discoveryUnfamiliar ? "Ranks by curated features with no positive evidence on titles in your library." : "Ranks untracked titles by curated evidence coverage."}
            </p>
            <label className="mt-3 block text-sm">Filter by feature
              <select value={discoveryFeature} disabled={busy} onChange={event => void filterDiscovery(event.target.value)}
                className="ml-2 rounded-lg border border-white/10 bg-slate-950 p-2">
                <option value="">All features</option>
                {preferences.map(feature => <option key={feature.key} value={feature.key}>{feature.name}</option>)}
              </select>
            </label>
            {selectedFeature && (
              <div className="mt-2 flex flex-wrap items-center gap-3" role="group" aria-label={selectedFeature.name + " preference for Personal Match"}>
                <button type="button" disabled={busy} aria-pressed={selectedFeature.weight === 1}
                  onClick={() => void save(selectedFeature, false, 1)} className="text-xs underline disabled:opacity-50">Prefer {selectedFeature.name}</button>
                <button type="button" disabled={busy} aria-pressed={selectedFeature.weight === -1}
                  onClick={() => void save(selectedFeature, false, -1)} className="text-xs underline disabled:opacity-50">Avoid {selectedFeature.name}</button>
                {selectedFeature.weight !== null && (
                  <button type="button" disabled={busy} onClick={() => void save(selectedFeature, true)}
                    className="text-xs underline disabled:opacity-50">Clear {selectedFeature.name} preference</button>
                )}
              </div>
            )}
            {discovery.items.length === 0 && <p className="mt-2 text-sm text-slate-400">No eligible discovery titles yet.</p>}
            <ul className="mt-3 space-y-2">
              {discovery.items.map(title => <li key={title.animeId} className="rounded-xl bg-white/5 p-3">
                <p className="text-sm font-medium">{title.title}</p>
                <p className="text-xs text-slate-400">{title.evidenceCount} supported {title.evidenceCount === 1 ? "feature" : "features"}</p>
                {title.unfamiliarCount !== undefined && <p className="text-xs text-slate-400">
                  {title.unfamiliarCount} {title.unfamiliarCount === 1 ? "feature" : "features"} missing from your library
                </p>}
                {title.featuredEvidence?.length ? (
                  <ul aria-label={"Explore features for " + title.title} className="mt-1 flex flex-wrap gap-2">
                    {title.featuredEvidence.map(feature => (
                      <li key={feature.key}>
                        <button type="button" disabled={busy || discoveryFeature === feature.key}
                          aria-label={"Explore " + feature.name} onClick={() => void filterDiscovery(feature.key)}
                          className="text-xs underline disabled:no-underline">
                          {feature.name}
                        </button>
                      </li>
                    ))}
                  </ul>
                ) : title.supportedFeatures?.length ? (
                  <p className="mt-1 text-xs text-slate-300">{title.supportedFeatures.join(" · ")}</p>
                ) : null}
                <button type="button" disabled={busy} aria-label={"Plan to watch " + title.title}
                  onClick={() => void planTitle(title)} className="mr-3 mt-2 text-xs underline disabled:opacity-50">Plan to watch</button>
                <button type="button" disabled={busy} aria-label={"Hide discovery title " + title.title} onClick={() => void setDismissed(title, true)}
                  className="mt-2 text-xs underline disabled:opacity-50">Hide title</button>
              </li>)}
            </ul>
            {discovery.nextCursor && (
              <button type="button" disabled={busy} onClick={() => {
                if (discovery.nextCursor) void loadMoreDiscovery(discovery.nextCursor);
              }} className="mt-2 text-xs underline disabled:opacity-50">Load more discovery titles</button>
            )}
          </section>
        )}
      </div>
      <p className="mt-4 text-xs text-slate-500">Compatibility scores are not probabilities. Matches consider up to 500 untracked titles and 1,000 recent series ratings. Unsupported or spoiler-sensitive observations are excluded. Rating-derived signal strength is an algorithmic weight, not a probability.</p>
    </div>}
  </section>;
}
