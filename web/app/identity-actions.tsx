"use client";
import { useEffect, useState } from "react";
import { ApiClient, StaleSessionError } from "../lib/api";
import { parseChallengeFragment, type Challenge } from "../lib/challenge";

function stale(error: unknown) {
  return (
    error instanceof StaleSessionError ||
    (error instanceof Error && error.name === "AbortError")
  );
}
export function RecoveryFlow({
  client,
  available,
  signedIn,
  onReset,
  onVerified,
}: {
  client: ApiClient;
  available: boolean;
  signedIn: boolean;
  onReset: () => void;
  onVerified: () => void;
}) {
  const [challenge, setChallenge] = useState<Challenge | null>(null),
    [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false);
  const [email, setEmail] = useState(""),
    [next, setNext] = useState(""),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  useEffect(() => {
    const read = () => {
      const hash = window.location.hash,
        params = new URLSearchParams(hash.slice(1));
      if (!params.has("token") && !params.has("action")) return;
      const proof = parseChallengeFragment(hash);
      // Replace the current history entry before any proof operation; never put
      // proof in localStorage, sessionStorage, hidden inputs, or request URLs.
      window.history.replaceState(
        null,
        "",
        window.location.pathname + window.location.search,
      );
      setChallenge(proof);
      setNotice("");
      setError(
        proof ? "" : "This link is invalid or expired. Request a new email.",
      );
    };
    read();
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, []);
  async function request(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await client.request("/auth/password-reset/request", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      setNotice(
        "If this account can receive recovery email, a link will arrive. Wait at least one minute before trying again.",
      );
    } catch (error) {
      if (!stale(error))
        setError(
          error instanceof Error
            ? error.message
            : "Could not request recovery email",
        );
    } finally {
      setEmail("");
      setBusy(false);
    }
  }
  async function confirm(event?: React.FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!challenge) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const reset = challenge.action === "reset-password";
      await client.request(
        reset
          ? "/auth/password-reset/confirm"
          : "/auth/email-verification/confirm",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(
            reset
              ? { token: challenge.token, newPassword: next }
              : { token: challenge.token },
          ),
        },
      );
      setChallenge(null);
      setOpen(false);
      if (reset) onReset();
      else {
        setNotice("Email verified. You can sign in normally.");
        onVerified();
      }
    } catch (error) {
      if (!stale(error))
        setError(
          error instanceof Error
            ? error.message
            : "Could not confirm this link",
        );
    } finally {
      setNext("");
      setBusy(false);
    }
  }
  if (!challenge && !open && !error && !notice && (!available || signedIn))
    return null;
  return (
    <section
      aria-label="Account recovery"
      className="mt-5 rounded-2xl border border-white/10 bg-white/[.03] p-5"
    >
      {challenge ? (
        <>
          <h2 className="font-semibold">
            {challenge.action === "reset-password"
              ? "Reset your password"
              : "Confirm your email"}
          </h2>
          {challenge.action === "reset-password" ? (
            <form
              onSubmit={(event) => void confirm(event)}
              className="mt-3 flex flex-wrap items-end gap-3"
            >
              <p className="w-full text-sm text-slate-400">
                Completing this link signs out all sessions for the account that
                requested it. Sign in again with the replacement password.
              </p>
              <label className="text-sm text-slate-300">
                Recovery new password
                <input
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={12}
                  maxLength={1024}
                  value={next}
                  onChange={(event) => setNext(event.target.value)}
                  disabled={busy}
                  className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-3"
                />
              </label>
              <button
                disabled={busy}
                className="rounded-lg bg-violet-300 px-4 py-3 text-slate-950 disabled:opacity-50"
              >
                Reset password
              </button>
            </form>
          ) : (
            <>
              <p className="mt-2 text-sm text-slate-400">
                Confirm this action to verify the email address that received
                the link.
              </p>
              <button
                disabled={busy}
                onClick={() => void confirm()}
                className="mt-3 rounded-lg bg-violet-300 px-4 py-3 text-slate-950 disabled:opacity-50"
              >
                Verify email
              </button>
            </>
          )}
          <button
            disabled={busy}
            onClick={() => {
              setChallenge(null);
              setNext("");
              setError("");
            }}
            className="mt-3 text-sm text-slate-400 underline"
          >
            Discard link
          </button>
        </>
      ) : (
        available &&
        !signedIn && (
          <>
            <button
              disabled={busy}
              aria-expanded={open}
              onClick={() => {
                setOpen(!open);
                setError("");
                setNotice("");
                setEmail("");
              }}
              className="text-sm text-violet-200 underline"
            >
              Forgot password?
            </button>
            {open && (
              <form
                onSubmit={(event) => void request(event)}
                className="mt-3 flex flex-wrap items-end gap-3"
              >
                <label className="text-sm text-slate-300">
                  Recovery email
                  <input
                    type="email"
                    autoComplete="email"
                    required
                    maxLength={254}
                    value={email}
                    onChange={(event) => setEmail(event.target.value)}
                    disabled={busy}
                    className="mt-1 block rounded-lg border border-white/10 bg-slate-950 p-3"
                  />
                </label>
                <button
                  disabled={busy}
                  className="rounded-lg bg-violet-300 px-4 py-3 text-slate-950 disabled:opacity-50"
                >
                  Request recovery email
                </button>
              </form>
            )}
          </>
        )
      )}
      {notice && (
        <p role="status" className="mt-3 text-sm text-emerald-200">
          {notice}
        </p>
      )}
      {error && (
        <p role="alert" className="mt-3 text-sm text-rose-300">
          {error}
        </p>
      )}
    </section>
  );
}
export function VerificationRequest({
  client,
  verified,
  onError,
}: {
  client: ApiClient;
  verified: boolean;
  onError: (error: unknown) => void;
}) {
  const [busy, setBusy] = useState(false),
    [notice, setNotice] = useState("");
  async function send() {
    setBusy(true);
    setNotice("");
    try {
      await client.request("/auth/email-verification/request", {
        method: "POST",
      });
      setNotice(
        "Verification requested. Check your inbox; resend is limited to once per minute.",
      );
    } catch (error) {
      onError(error);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section
      aria-label="Email verification"
      className="mt-4 rounded-2xl border border-white/10 bg-white/[.03] p-5"
    >
      <p className="text-sm text-slate-300">
        {verified ? "Email verified" : "Email not verified"}
      </p>
      {!verified && (
        <button
          disabled={busy}
          onClick={() => void send()}
          className="mt-2 text-sm text-violet-200 underline disabled:opacity-50"
        >
          Send verification email
        </button>
      )}
      {notice && (
        <p role="status" className="mt-3 text-sm text-slate-400">
          {notice}
        </p>
      )}
    </section>
  );
}
