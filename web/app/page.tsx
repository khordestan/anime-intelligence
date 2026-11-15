"use client";
import { useCallback, useEffect, useMemo, useState } from "react";
import SessionManager from "./session-manager";
import PasswordManager from "./password-manager";
import { RecoveryFlow, VerificationRequest } from "./identity-actions";

import {
  ApiClient,
  ApiError,
  StaleSessionError,
  type Session,
  type User,
} from "../lib/api";

const API =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";
type Anime = {
  id: string;
  title: string;
  releaseYear?: number;
  mediaType: string;
};
type Entry = {
  id: string;
  animeId: string;
  title: string;
  watchStatus: string;
  episodesWatched: number;
  totalEpisodes?: number;
  rating?: number;
};
const statuses = [
  "ALL",
  "PLANNING",
  "WATCHING",
  "COMPLETED",
  "ON_HOLD",
  "DROPPED",
];
const labels: Record<string, string> = {
  ALL: "All",
  PLANNING: "Planning",
  WATCHING: "Watching",
  COMPLETED: "Completed",
  ON_HOLD: "On hold",
  DROPPED: "Dropped",
};

export default function Home() {
  const [library, setLibrary] = useState<Entry[]>([]),
    [catalog, setCatalog] = useState<Anime[]>([]),
    [query, setQuery] = useState(""),
    [filter, setFilter] = useState("ALL"),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [client] = useState(() => new ApiClient(API));
  const [identityAvailable, setIdentityAvailable] = useState(false);
  const [mode, setMode] = useState<"local" | "postgres" | null>(null);
  const [session, setSession] = useState<Session | null>(null);
  const [email, setEmail] = useState(""),
    [password, setPassword] = useState("");
  const [register, setRegister] = useState(false),
    [authBusy, setAuthBusy] = useState(false);
  const [busy, setBusy] = useState(false);
  const changeSession = useCallback(
    (next: Session | null) => {
      client.setSession(next);
      setSession(next);
      setLibrary([]);
      setNotice("");
      setPassword("");
      setEmail("");
      setRegister(false);
      setError("");
      setQuery("");
      setFilter("ALL");
      setBusy(false);
    },
    [client],
  );
  const handleError = useCallback(
    (e: unknown) => {
      if (
        e instanceof StaleSessionError ||
        (e instanceof Error && e.name === "AbortError")
      )
        return;
      if (e instanceof ApiError && e.status === 401) changeSession(null);
      setError(e instanceof Error ? e.message : "Could not connect to API");
    },
    [changeSession],
  );
  useEffect(() => {
    let active = true;
    client
      .request<{
        authenticationRequired?: boolean;
        identityDeliveryAvailable?: boolean;
      }>("")
      .then((meta) => {
        if (active) {
          setMode(meta.authenticationRequired ? "postgres" : "local");
          setIdentityAvailable(!!meta.identityDeliveryAvailable);
        }
      })
      .catch((e) => {
        if (active) {
          handleError(e);
          setLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, [client, handleError]);
  async function refreshVerifiedUser() {
    if (!session) return;
    try {
      const user = await client.request<User>("/auth/me");
      setSession((current) =>
        current && current.user.id === user.id ? { ...current, user } : current,
      );
    } catch (error) {
      handleError(error);
    }
  }
  const refresh = useCallback(async () => {
    if (mode === null) return;
    setLoading(true);
    try {
      const [c, l] = await Promise.all([
        client.request<{ items: Anime[] }>("/anime"),
        mode === "local" || session
          ? client.request<{ items: Entry[] }>("/tracker")
          : Promise.resolve({ items: [] as Entry[] }),
      ]);
      setCatalog(c.items ?? []);
      setLibrary(l.items ?? []);
      setError("");
    } catch (e) {
      handleError(e);
    } finally {
      setLoading(false);
    }
  }, [client, mode, session, handleError]);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  useEffect(() => {
    if (!session) return;
    const remaining = new Date(session.expiresAt).getTime() - Date.now();
    const id = setTimeout(
      () => {
        changeSession(null);
        setError("Your session expired. Sign in again.");
      },
      Math.max(0, remaining),
    );
    return () => clearTimeout(id);
  }, [session, changeSession]);
  async function authenticate(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setAuthBusy(true);
    setError("");
    try {
      const next = await client.request<Session>(
        register ? "/auth/register" : "/auth/login",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, password }),
        },
      );
      changeSession(next);
    } catch (e) {
      handleError(e);
    } finally {
      setPassword("");
      setAuthBusy(false);
    }
  }
  async function logout() {
    setAuthBusy(true);
    try {
      await client.request<void>("/auth/logout", { method: "POST" });
      changeSession(null);
    } catch (e) {
      changeSession(null);
      if (!(e instanceof ApiError && e.status === 401))
        setError(
          "Signed out of this tab. Server revocation could not be confirmed.",
        );
    } finally {
      setAuthBusy(false);
    }
  }
  async function mutate(path: string, method: string, body: object) {
    setBusy(true);
    try {
      await client.request(path, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      setQuery("");
      await refresh();
    } catch (e) {
      handleError(e);
    } finally {
      setBusy(false);
    }
  }
  const filtered = useMemo(
    () => library.filter((x) => filter === "ALL" || x.watchStatus === filter),
    [library, filter],
  );
  const results = useMemo(
    () =>
      catalog
        .filter(
          (x) =>
            x.title.toLowerCase().includes(query.toLowerCase()) &&
            !library.some((e) => e.animeId === x.id),
        )
        .slice(0, 8),
    [catalog, library, query],
  );
  async function add(a: Anime) {
    await mutate("/tracker", "POST", {
      animeId: a.id,
      watchStatus: "PLANNING",
      episodesWatched: 0,
    });
  }
  async function patch(e: Entry, body: object) {
    await mutate("/tracker/" + e.id, "PATCH", body);
  }
  async function rate(e: Entry, score: number) {
    await mutate("/ratings/" + e.animeId, "PUT", { score });
  }
  const canEdit = mode === "local" || !!session;
  const episodes = library.reduce((n, e) => n + e.episodesWatched, 0);
  return (
    <main className="mx-auto min-h-screen max-w-6xl px-5 py-10">
      <header className="flex items-center justify-between border-b border-white/10 pb-6">
        <div>
          <p className="text-xs tracking-[.3em] text-violet-300">
            ANIME INTELLIGENCE
          </p>
          <h1 className="mt-2 text-3xl font-bold">Your anime universe</h1>
        </div>
        <div className="text-right">
          <span className="rounded-full border border-emerald-400/30 px-3 py-2 text-xs text-emerald-300">
            {mode === "postgres" ? "Account development" : "Local development"}
          </span>
          {session && (
            <div className="mt-3 text-xs text-slate-400">
              <span>{session.user.email}</span>
              <button
                disabled={authBusy}
                onClick={() => void logout()}
                className="ml-3 underline disabled:opacity-50"
              >
                Sign out
              </button>
            </div>
          )}
        </div>
      </header>
      {notice && (
        <p
          role="status"
          className="mt-5 rounded-xl border border-emerald-400/30 p-3 text-sm text-emerald-200"
        >
          {notice}
        </p>
      )}
      {mode === "postgres" && !session && (
        <section className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5">
          <h2 className="text-xl font-semibold">
            {register ? "Create your account" : "Sign in to your library"}
          </h2>
          <p className="mt-2 text-sm text-slate-400">
            Your account session stays in this tab. Reloading the page signs you
            out of the dashboard.
          </p>
          <form
            onSubmit={(e) => void authenticate(e)}
            className="mt-4 grid gap-3 sm:grid-cols-3"
          >
            <label className="text-sm text-slate-300">
              Email
              <input
                aria-label="Email"
                type="email"
                autoComplete="username"
                required
                maxLength={254}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={authBusy}
                className="mt-1 w-full rounded-lg border border-white/10 bg-slate-950 p-3"
              />
            </label>
            <label className="text-sm text-slate-300">
              Password
              <input
                aria-label="Password"
                type="password"
                autoComplete={register ? "new-password" : "current-password"}
                required
                minLength={register ? 12 : 1}
                maxLength={1024}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={authBusy}
                className="mt-1 w-full rounded-lg border border-white/10 bg-slate-950 p-3"
              />
            </label>
            <button
              disabled={authBusy}
              className="self-end rounded-lg bg-violet-300 px-4 py-3 text-slate-950 disabled:opacity-50"
            >
              {authBusy
                ? "Please wait…"
                : register
                  ? "Create account"
                  : "Sign in"}
            </button>
          </form>
          <button
            disabled={authBusy}
            onClick={() => {
              setRegister(!register);
              setPassword("");
              setError("");
            }}
            className="mt-3 text-sm text-violet-300 underline"
          >
            {register
              ? "Already have an account? Sign in"
              : "Create an account"}
          </button>
        </section>
      )}
      {session && (
        <SessionManager
          key={`sessions:${session.user.id}`}
          client={client}
          onError={handleError}
        />
      )}
      {session && (
        <PasswordManager
          key={`password:${session.user.id}`}
          client={client}
          onChanged={() => {
            changeSession(null);
            setNotice("Password changed. Sign in with your new password.");
          }}
          onError={handleError}
        />
      )}
      {mode === "postgres" && (
        <RecoveryFlow
          key={`recovery:${session?.user.id ?? "anonymous"}`}
          client={client}
          available={identityAvailable}
          signedIn={!!session}
          onReset={() => {
            changeSession(null);
            setNotice("Password reset. Sign in with your new password.");
          }}
          onVerified={() => void refreshVerifiedUser()}
        />
      )}
      {session && identityAvailable && (
        <VerificationRequest
          key={`verification:${session.user.id}`}
          client={client}
          verified={!!session.user.emailVerified}
          onError={handleError}
        />
      )}
      <section className="mt-8 grid gap-4 sm:grid-cols-3">
        <Stat label="In your library" value={library.length} />
        <Stat
          label="Currently watching"
          value={library.filter((e) => e.watchStatus === "WATCHING").length}
        />
        <Stat label="Episodes watched" value={episodes} />
      </section>
      <section className="mt-8 rounded-2xl border border-white/10 bg-white/[.03] p-5">
        <h2 className="font-semibold">Add anime</h2>
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search starter catalog…"
          className="mt-3 w-full rounded-xl border border-white/10 bg-slate-950 px-4 py-3 outline-none focus:border-violet-400"
        />
        {query && (
          <div className="mt-3 space-y-2">
            {results.map((a) => (
              <div
                key={a.id}
                className="flex items-center justify-between rounded-xl bg-white/5 p-3"
              >
                <span>
                  {a.title}{" "}
                  <small className="text-slate-500">{a.releaseYear}</small>
                </span>
                <button
                  disabled={!canEdit || busy}
                  onClick={() => void add(a)}
                  className="rounded-lg bg-violet-400/20 px-3 py-2 text-sm text-violet-200"
                >
                  Add
                </button>
              </div>
            ))}
          </div>
        )}
      </section>
      <section className="mt-10">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <h2 className="text-2xl font-semibold">My Library</h2>
          <button
            onClick={() => void refresh()}
            className="text-sm text-slate-400 underline"
          >
            Refresh
          </button>
        </div>
        <div className="mt-4 flex flex-wrap gap-2">
          {statuses.map((s) => (
            <button
              key={s}
              onClick={() => setFilter(s)}
              className={`rounded-full px-4 py-2 text-xs ${filter === s ? "bg-violet-300 text-slate-950" : "border border-white/10 text-slate-400"}`}
            >
              {labels[s]}
            </button>
          ))}
        </div>
        {error && (
          <p
            role="alert"
            className="mt-4 rounded-xl border border-rose-400/30 p-3 text-sm text-rose-300"
          >
            {error}
          </p>
        )}
        {!canEdit && mode === "postgres" && (
          <p className="mt-5 text-sm text-slate-400">
            Sign in to add anime and view your private library.
          </p>
        )}
        {loading ? (
          <p className="mt-6 text-slate-500">Loading…</p>
        ) : (
          <div className="mt-5 grid gap-3 md:grid-cols-2">
            {filtered.map((e) => (
              <article
                key={e.id}
                className="rounded-2xl border border-white/10 bg-white/[.03] p-5"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h3 className="font-semibold">{e.title}</h3>
                    <p className="mt-1 text-xs text-slate-500">
                      {e.episodesWatched}
                      {e.totalEpisodes ? "/" + e.totalEpisodes : ""} episodes
                      watched
                    </p>
                  </div>
                  <select
                    value={e.watchStatus}
                    disabled={busy}
                    onChange={(x) =>
                      void patch(e, { watchStatus: x.target.value })
                    }
                    className="rounded-lg border border-white/10 bg-slate-900 p-2 text-xs"
                  >
                    {statuses.slice(1).map((s) => (
                      <option key={s} value={s}>
                        {labels[s]}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="mt-4 flex items-center justify-between gap-2">
                  <label className="text-xs text-slate-400">Rating</label>
                  <select
                    disabled={busy}
                    aria-label={"Rating " + e.title}
                    value={e.rating ?? ""}
                    onChange={(x) =>
                      x.target.value && void rate(e, Number(x.target.value))
                    }
                    className="rounded-lg border border-white/10 bg-slate-900 p-2 text-xs"
                  >
                    <option value="">Not rated</option>
                    {[1, 2, 3, 4, 5].map((n) => (
                      <option key={n} value={n}>
                        {n} / 5
                      </option>
                    ))}
                  </select>
                  <button
                    disabled={busy}
                    onClick={() =>
                      void patch(e, { episodesWatched: e.episodesWatched + 1 })
                    }
                    className="rounded-lg border border-white/10 px-3 py-2 text-xs"
                  >
                    +1 episode
                  </button>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
      <footer className="mt-14 border-t border-white/10 pt-5 text-xs text-slate-600">
        Anime Intelligence · Development foundation ·{" "}
        {mode === "postgres" ? "Private account library" : "Local demo profile"}
      </footer>
    </main>
  );
}
function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[.03] p-5">
      <p className="text-xs text-slate-500">{label}</p>
      <p className="mt-2 text-3xl font-semibold tabular-nums">{value}</p>
    </div>
  );
}
