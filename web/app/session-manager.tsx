"use client";

import { useState } from "react";
import { ApiClient, ApiError } from "../lib/api";

type SessionInfo = {
  id: string;
  createdAt: string;
  expiresAt: string;
  lastUsedAt: string;
  current: boolean;
};
type SessionPage = { items: SessionInfo[]; hasMore: boolean };

export default function SessionManager({
  client,
  onError,
}: {
  client: ApiClient;
  onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<SessionInfo[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");

  async function load() {
    const page = await client.request<SessionPage>("/auth/sessions");
    setItems(page.items);
    setHasMore(page.hasMore);
  }
  async function refresh() {
    setBusy(true);
    setNotice("");
    try {
      await load();
    } catch (error) {
      onError(error);
    } finally {
      setBusy(false);
    }
  }
  async function revoke(id: string) {
    setBusy(true);
    setNotice("");
    try {
      try {
        await client.request<void>("/auth/sessions/" + id, {
          method: "DELETE",
        });
      } catch (error) {
        if (!(error instanceof ApiError && error.status === 404)) throw error;
      }
      await load();
      setNotice("Session signed out.");
    } catch (error) {
      onError(error);
    } finally {
      setBusy(false);
    }
  }
  async function revokeOthers() {
    setBusy(true);
    setNotice("");
    try {
      const result = await client.request<{ revoked: number }>(
        "/auth/sessions/revoke-others",
        { method: "POST" },
      );
      await load();
      setNotice(
        `Signed out ${result.revoked} other ${result.revoked === 1 ? "session" : "sessions"}.`,
      );
    } catch (error) {
      onError(error);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section
      aria-label="Active sessions"
      className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5"
    >
      <button
        aria-expanded={open}
        aria-controls="session-list"
        disabled={busy}
        onClick={() => {
          setOpen(!open);
          if (!open) void refresh();
        }}
        className="text-sm text-violet-200 underline disabled:opacity-50"
      >
        {open ? "Hide sessions" : "Manage sessions"}
      </button>
      {open && (
        <div id="session-list" className="mt-4">
          <h2 className="font-semibold">Active sessions</h2>
          <p className="mt-2 text-sm text-slate-400">
            Sign out another login without ending this session. Reloading a tab
            does not revoke its server session.
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            <button
              disabled={busy}
              onClick={() => void refresh()}
              className="rounded-lg border border-white/10 px-3 py-2 text-sm disabled:opacity-50"
            >
              Refresh sessions
            </button>
            <button
              disabled={
                busy || (!hasMore && !items.some((item) => !item.current))
              }
              onClick={() => void revokeOthers()}
              className="rounded-lg border border-white/10 px-3 py-2 text-sm disabled:opacity-50"
            >
              Sign out other sessions
            </button>
          </div>
          <p role="status" className="mt-3 text-sm text-slate-400">
            {busy ? "Loading sessions…" : notice}
          </p>
          {hasMore && (
            <p className="mt-3 text-sm text-amber-200">
              Showing the current session and the newest 99 others. Sign out
              other sessions also includes older logins.
            </p>
          )}
          <ul className="mt-3 space-y-3">
            {items.map((item) => (
              <li
                key={item.id}
                className="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-white/5 p-3"
              >
                <div>
                  <p className="text-sm">
                    {item.current
                      ? "Current session"
                      : `Session ${item.id.slice(-8)}`}
                  </p>
                  <p className="mt-1 text-xs text-slate-400">
                    Created {new Date(item.createdAt).toLocaleString()} · Last
                    used {new Date(item.lastUsedAt).toLocaleString()}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    Expires {new Date(item.expiresAt).toLocaleString()}
                  </p>
                </div>
                {!item.current && (
                  <button
                    disabled={busy}
                    aria-label={`Revoke session ${item.id}`}
                    onClick={() => void revoke(item.id)}
                    className="rounded-lg border border-rose-400/30 px-3 py-2 text-sm text-rose-200 disabled:opacity-50"
                  >
                    Revoke
                  </button>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
