"use client";
import { useState } from "react";
import { ApiClient, ApiError } from "../lib/api";

export default function PasswordManager({
  client,
  onChanged,
  onError,
}: {
  client: ApiClient;
  onChanged: () => void;
  onError: (error: unknown) => void;
}) {
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false);
  const [current, setCurrent] = useState(""),
    [next, setNext] = useState(""),
    [error, setError] = useState("");
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await client.request<void>("/auth/password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ currentPassword: current, newPassword: next }),
      });
      onChanged();
    } catch (error) {
      if (
        error instanceof ApiError &&
        (error.code === "invalid_current_password" ||
          error.code === "invalid_password")
      )
        setError(error.message);
      else onError(error);
    } finally {
      setCurrent("");
      setNext("");
      setBusy(false);
    }
  }
  return (
    <section
      aria-label="Password settings"
      className="mt-4 rounded-2xl border border-white/10 bg-white/[.03] p-5"
    >
      <button
        disabled={busy}
        aria-expanded={open}
        aria-controls="password-form"
        onClick={() => {
          setOpen(!open);
          setCurrent("");
          setNext("");
          setError("");
        }}
        className="text-sm text-violet-200 underline disabled:opacity-50"
      >
        {open ? "Hide password settings" : "Change password"}
      </button>
      {open && (
        <form
          id="password-form"
          onSubmit={(event) => void submit(event)}
          className="mt-4 grid gap-3 sm:grid-cols-3"
        >
          <p className="text-sm text-slate-400 sm:col-span-3">
            Changing your password signs out every session, including this one.
            Sign in again with your new password.
          </p>
          <label className="text-sm text-slate-300">
            Current password
            <input
              type="password"
              autoComplete="current-password"
              required
              maxLength={1024}
              value={current}
              onChange={(event) => setCurrent(event.target.value)}
              disabled={busy}
              className="mt-1 w-full rounded-lg border border-white/10 bg-slate-950 p-3"
            />
          </label>
          <label className="text-sm text-slate-300">
            New password
            <input
              type="password"
              autoComplete="new-password"
              required
              minLength={12}
              maxLength={1024}
              value={next}
              onChange={(event) => setNext(event.target.value)}
              disabled={busy}
              className="mt-1 w-full rounded-lg border border-white/10 bg-slate-950 p-3"
            />
          </label>
          <button
            disabled={busy}
            className="self-end rounded-lg bg-violet-300 px-4 py-3 text-slate-950 disabled:opacity-50"
          >
            {busy ? "Changing password…" : "Update password"}
          </button>
          {error && (
            <p role="alert" className="text-sm text-rose-300 sm:col-span-3">
              {error}
            </p>
          )}
        </form>
      )}
    </section>
  );
}
