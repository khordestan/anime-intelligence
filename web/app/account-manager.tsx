"use client";
import { useState } from "react";
import { ApiClient, ApiError } from "../lib/api";
import { downloadJSON } from "../lib/download";

export default function AccountManager({
  client,
  onDeleted,
  onError,
}: {
  client: ApiClient;
  onDeleted: () => void;
  onError: (error: unknown) => void;
}) {
  const [busy, setBusy] = useState(false),
    [open, setOpen] = useState(false);
  const [password, setPassword] = useState(""),
    [confirmed, setConfirmed] = useState(false);
  const [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  async function download() {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      // The shared client rejects late responses after an account switch.
      const data = await client.request<unknown>("/auth/export");
      downloadJSON("anime-intelligence-account.json", data);
      setNotice(
        "Export downloaded. Keep this file private: it contains your account and library data.",
      );
    } catch (error) {
      onError(error);
    } finally {
      setBusy(false);
    }
  }
  async function remove(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!confirmed) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await client.request<void>("/auth/account", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
      onDeleted();
    } catch (error) {
      if (
        error instanceof ApiError &&
        error.code === "invalid_current_password"
      )
        setError(error.message);
      else onError(error);
    } finally {
      setPassword("");
      setConfirmed(false);
      setBusy(false);
    }
  }
  return (
    <section
      aria-label="Account data"
      className="mt-4 rounded-2xl border border-white/10 bg-white/[.03] p-5"
    >
      <h2 className="font-semibold">Account data</h2>
      <p className="mt-2 text-sm text-slate-400">
        Download your private data before deleting your account.
      </p>
      <div className="mt-3 flex flex-wrap gap-4">
        <button
          disabled={busy}
          onClick={() => void download()}
          className="text-sm text-violet-200 underline disabled:opacity-50"
        >
          Download account export
        </button>
        <button
          disabled={busy}
          aria-expanded={open}
          aria-controls="delete-account-form"
          onClick={() => {
            setOpen(!open);
            setPassword("");
            setConfirmed(false);
            setError("");
            setNotice("");
          }}
          className="text-sm text-rose-300 underline disabled:opacity-50"
        >
          {open ? "Cancel account deletion" : "Delete account"}
        </button>
      </div>
      {open && (
        <form
          id="delete-account-form"
          onSubmit={(event) => void remove(event)}
          className="mt-4 grid gap-3"
        >
          <p className="text-sm text-rose-200">
            Deletion permanently removes your account, library, ratings,
            progress, preferences and notifications. It signs out all sessions
            and invalidates recovery links. This cannot be undone.
          </p>
          <label className="text-sm text-slate-300">
            Deletion password
            <input
              type="password"
              autoComplete="current-password"
              required
              maxLength={1024}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={busy}
              className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-3"
            />
          </label>
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input
              type="checkbox"
              required
              checked={confirmed}
              onChange={(event) => setConfirmed(event.target.checked)}
              disabled={busy}
            />
            I understand that deleting my account is permanent.
          </label>
          <button
            disabled={busy || !confirmed}
            className="justify-self-start rounded-lg bg-rose-300 px-4 py-3 text-slate-950 disabled:opacity-50"
          >
            Permanently delete my account
          </button>
        </form>
      )}
      {error && (
        <p role="alert" className="mt-3 text-sm text-rose-300">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="mt-3 text-sm text-emerald-300">
          {notice}
        </p>
      )}
    </section>
  );
}
