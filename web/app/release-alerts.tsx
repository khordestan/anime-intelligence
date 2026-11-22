"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Follow = { animeId: string; title: string; track: string; scope?: "ANIME" | "FRANCHISE"; createdAt: string };
type Notice = { id: string; title: string; body: string; createdAt: string; readAt?: string };
const tracks = ["SUB", "DUB", "RAW", "UNKNOWN", "ALL"];

export default function ReleaseAlerts({ client, library, onError }: {
  client: ApiClient;
  library: { animeId: string; title: string }[];
  onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [follows, setFollows] = useState<Follow[]>([]);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const unreadFilter = useRef(false);
  const [unreadCount, setUnreadCount] = useState<number | null>(null);
  const [notices, setNotices] = useState<Notice[]>([]);
  const [cursor, setCursor] = useState("");
  const [nextCursor, setNextCursor] = useState("");
  const [catchingUp, setCatchingUp] = useState(false);
  const [preferences, setPreferences] = useState<Record<string, string>>({});
  const [scopes, setScopes] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const alive = useRef(true);
  const revision = useRef(0);
  const inFlight = useRef(false);
  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; revision.current++; };
  }, []);
  const read = useCallback(async (current: number, pageCursor = "", onlyUnread = unreadFilter.current) => {
    const params = new URLSearchParams();
    if (pageCursor) params.set("cursor", pageCursor);
    if (onlyUnread) params.set("unreadOnly", "true");
    const query = params.toString();
    const [f, n] = await Promise.all([
      client.request<{ items: Follow[] }>("/follows"),
      client.request<{ items: Notice[]; unreadCount?: number; nextCursor?: string; materialization?: { created: number; hasMore: boolean } }>("/notifications" + (query ? "?" + query : "")),
    ]);
    if (!alive.current || current !== revision.current) return;
    unreadFilter.current = onlyUnread;
    setUnreadOnly(onlyUnread);
    setFollows(f.items);
    setNotices(n.items);
    setUnreadCount(n.unreadCount ?? null);
    setCursor(pageCursor); setNextCursor(n.nextCursor ?? "");
    setCatchingUp(!pageCursor && n.materialization?.hasMore === true);
  }, [client]);
  const refresh = useCallback(async (pageCursor = "", onlyUnread = unreadFilter.current, collapsed = false) => {
    if (inFlight.current) return;
    inFlight.current = true;
    const current = ++revision.current;
    setBusy(true);
    setMessage("");
    try {
      if (collapsed) {
        const page = await client.request<{ unreadCount?: number }>("/notifications?unreadOnly=true");
        if (alive.current && current === revision.current) setUnreadCount(page.unreadCount ?? null);
      } else {
        await read(current, pageCursor, onlyUnread);
      }
    } catch (error) {
      if (!collapsed && alive.current && current === revision.current) {
        setMessage("Could not refresh release alerts. Try again.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) {
        inFlight.current = false;
        setBusy(false);
      }
    }
  }, [client, read, onError]);
  useEffect(() => {
    if (cursor) return;
    if (open) void refresh();
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") void refresh("", unreadFilter.current, !open);
    }, 60_000);
    return () => clearInterval(timer);
  }, [open, refresh, cursor]);
  async function mutate(path: string, method: string, body?: object) {
    if (inFlight.current) return;
    inFlight.current = true;
    const current = ++revision.current;
    setBusy(true);
    setMessage("");
    try {
      await client.request<void>(path, {
        method,
        headers: { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      if (!alive.current || current !== revision.current) return;
      await read(current, cursor);
      if (alive.current && current === revision.current) setMessage("Release alerts updated.");
    } catch (error) {
      if (alive.current && current === revision.current) {
        setMessage("Could not update release alerts. Try again.");
        onError(error);
      }
    } finally {
      if (alive.current && current === revision.current) {
        inFlight.current = false;
        setBusy(false);
      }
    }
  }
  const titles = new Map(library.map(item => [item.animeId, item.title]));
  for (const follow of follows) titles.set(follow.animeId, follow.title);
  return (
    <section aria-label="Release alerts" className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5">
      <button aria-expanded={open} aria-controls="release-alert-panel" onClick={() => setOpen(!open)}
        className="text-sm text-violet-200 underline">
        {open ? "Hide release alerts" : "Manage release alerts"}
      </button>
      {!open && unreadCount !== null && unreadCount > 0 && (
        <span role="status" className="ml-3 text-xs text-violet-200">Unread alerts: {unreadCount}</span>
      )}
      {open && <div id="release-alert-panel" className="mt-4">
        <h2 className="font-semibold">Release alerts</h2>
        <p className="mt-2 text-sm text-slate-400">Checks every minute while this dashboard tab is visible and you are on the latest page. Episode notices require confirmed releases; linked-season follows also include new announcements. Scheduled times can change.</p>
        <button disabled={busy} onClick={() => void refresh()} className="mt-3 text-sm underline disabled:opacity-50">{cursor ? "Back to latest alerts" : "Refresh alerts"}</button>
        <p role="status" className="mt-2 text-sm text-slate-400">{busy ? "Loading release alerts…" : message}</p>
        {!cursor && catchingUp && <p className="mt-2 text-sm text-amber-200">More release alerts are waiting. Refresh alerts to continue catching up; each check adds up to 100 notifications.</p>}
        <h3 className="mt-5 font-medium">Followed releases</h3>
        {titles.size === 0 && <p className="mt-2 text-sm text-slate-400">Add an anime to your library to follow its releases.</p>}
        <ul className="mt-3 space-y-3">
          {Array.from(titles, ([id, title]) => {
            const follow = follows.find(item => item.animeId === id);
            return <li key={id} className="flex flex-wrap items-center gap-3 rounded-xl bg-white/5 p-3">
              <span className="grow text-sm">{title}</span>
              <select aria-label={"Release scope " + title} value={scopes[id] ?? follow?.scope ?? "ANIME"} disabled={busy}
                onChange={e => setScopes(current => ({ ...current, [id]: e.target.value }))}
                className="rounded-lg border border-white/10 bg-slate-900 p-2 text-xs">
                <option value="ANIME">This season</option><option value="FRANCHISE">All linked seasons</option>
              </select>
              <select aria-label={"Release track " + title} value={preferences[id] ?? follow?.track ?? "SUB"}
                disabled={busy} onChange={e => setPreferences(current => ({ ...current, [id]: e.target.value }))}
                className="rounded-lg border border-white/10 bg-slate-900 p-2 text-xs">
                {tracks.map(track => <option key={track} value={track}>{track === "ALL" ? "All tracks" : track}</option>)}
              </select>
              <button disabled={busy} aria-label={(follow ? "Save release track " : "Follow releases ") + title}
                onClick={() => void mutate("/follows/" + id, "PUT", { track: preferences[id] ?? follow?.track ?? "SUB", scope: scopes[id] ?? follow?.scope ?? "ANIME" })}
                className="text-xs text-violet-200 underline disabled:opacity-50">{follow ? "Save track" : "Follow releases"}</button>
              {follow && <button disabled={busy} aria-label={"Stop alerts " + title}
                onClick={() => void mutate("/follows/" + id + "?scope=" + (follow.scope ?? "ANIME"), "DELETE")} className="text-xs text-rose-200 underline disabled:opacity-50">Stop alerts</button>}
            </li>;
          })}
        </ul>
        <h3 className="mt-5 font-medium">{cursor ? "Notification history" : "Recent notifications"}</h3>
        {unreadCount !== null && <p className="mt-2 text-sm text-slate-400">Unread alerts: {unreadCount}</p>}
        {(unreadCount === null ? notices.length > 0 : unreadCount > 0) && (
          <button type="button" disabled={busy} onClick={() => void mutate("/notifications/read-all", "PATCH")}
            className="mt-2 text-xs underline disabled:opacity-50">Mark all alerts as read</button>
        )}
        {cursor && <p className="mt-2 text-sm text-slate-400">Automatic refresh is paused while viewing history. Return to the latest page to check for new releases.</p>}
        <label className="mt-2 flex items-center gap-2 text-sm">
          <input type="checkbox" checked={unreadOnly} disabled={busy}
            onChange={event => void refresh("", event.target.checked)} />
          Show unread alerts only
        </label>
        {!busy && notices.length === 0 && (
          <p className="mt-2 text-sm text-slate-400">{unreadOnly ? "No unread release alerts." : "No release notifications yet."}</p>
        )}
        <ul className="mt-3 space-y-3" aria-label="Release notifications">
          {notices.map(notice => <li key={notice.id} className="rounded-xl bg-white/5 p-3">
            <p className="text-sm font-medium">{notice.title} {!notice.readAt && <span className="ml-2 text-xs text-violet-200">Unread</span>}</p>
            <p className="mt-1 text-sm text-slate-300">{notice.body}</p>
            <p className="mt-1 text-xs text-slate-500">{new Date(notice.createdAt).toLocaleString()}</p>
            {!notice.readAt && <button disabled={busy} onClick={() => void mutate("/notifications/" + notice.id + "/read", "PATCH")}
              className="mt-2 text-xs underline disabled:opacity-50">Mark as read</button>}
          </li>)}
        </ul>
        {nextCursor && <button disabled={busy} onClick={() => void refresh(nextCursor)} className="mt-3 text-sm underline disabled:opacity-50">Older notifications</button>}
        <p className="mt-4 text-xs text-slate-500">Showing up to 1,000 follows and 100 notifications per page. Email and push alerts are not enabled.</p>
      </div>}
    </section>
  );
}
