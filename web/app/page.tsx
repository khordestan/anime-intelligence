"use client";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import SessionManager from "./session-manager";
import ProviderCredit from "./provider-credit";
import ReleaseAlerts from "./release-alerts";
import ReleaseCalendar from "./release-calendar";
import EpisodeEditor from "./episode-editor";
import RewatchManager from "./rewatch-manager";
import NotesEditor from "./notes-editor";
import CatalogDetails from "./catalog-details";
import CatalogFilters, { emptyCatalogFilters, type CatalogFilterValues } from "./catalog-filters";
import PersonalMatches from "./personal-matches";
import PasswordManager from "./password-manager";
import AccountManager from "./account-manager";
import { downloadJSON } from "../lib/download";
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
  exactProgress?: boolean;
  watchCycle?: number;
  totalEpisodes?: number;
  rating?: number;
};
type BulkStatusTarget = { id: string; expectedCycle: number };
const maxBulkStatusEntries = 25;

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
  const [libraryQuery, setLibraryQuery] = useState("");
  const [libraryRating, setLibraryRating] = useState("ALL");
  const [librarySort, setLibrarySort] = useState("DEFAULT");
  const [libraryProgress, setLibraryProgress] = useState("ALL");
  const [notice, setNotice] = useState("");
  const [searchedCatalog, setSearchedCatalog] = useState<Anime[] | null>(null);
  const [catalogFilters, setCatalogFilters] = useState<CatalogFilterValues>(emptyCatalogFilters);
  const [browseCatalog, setBrowseCatalog] = useState(false);
  const hasCatalogFilters = Object.values(catalogFilters).some(Boolean);
  const showCatalogResults = browseCatalog || Boolean(query.trim()) || hasCatalogFilters;
  const [catalogSearchBusy, setCatalogSearchBusy] = useState(false);
  const [catalogSearchError, setCatalogSearchError] = useState("");
  const [catalogResultLimit, setCatalogResultLimit] = useState(8);
  const [catalogPage, setCatalogPage] = useState<{ trail: string[]; next?: string }>({ trail: [] });
  const retryPage = useRef<string[] | null>(null);
  const catalogSearchRevision = useRef(0);
  const [client] = useState(() => new ApiClient(API));
  const [identityAvailable, setIdentityAvailable] = useState(false);
  const [mode, setMode] = useState<"local" | "postgres" | null>(null);
  const [session, setSession] = useState<Session | null>(null);
  const [email, setEmail] = useState(""),
    [password, setPassword] = useState("");
  const [register, setRegister] = useState(false),
    [authBusy, setAuthBusy] = useState(false);
  const [busy, setBusy] = useState(false);
  const [watchListPreview, setWatchListPreview] = useState<{ added: number; skipped: number } | null>(null);
  const [watchListFile, setWatchListFile] = useState<File | null>(null);
  const [showWatchListImport, setShowWatchListImport] = useState(false);
  const [bulkEntries, setBulkEntries] = useState<BulkStatusTarget[] | null>(null);
  const [bulkRating, setBulkRating] = useState("");
  const [bulkStatus, setBulkStatus] = useState("");
  const [removal, setRemoval] = useState<string | null>(null);
  const accountRevision = useRef(0);
  const refreshRevision = useRef(0);
  const changeSession = useCallback(
    (next: Session | null) => {
      accountRevision.current++;
      catalogSearchRevision.current++;
      setSearchedCatalog(null);
      setCatalogFilters(emptyCatalogFilters); setCatalogSearchBusy(false); setCatalogSearchError("");
      refreshRevision.current++;
      setRemoval(null);
      setBulkEntries(null);
      setBulkStatus("");
      setBulkRating("");
      setWatchListFile(null);
      setWatchListPreview(null);
      setShowWatchListImport(false);
      client.setSession(next);
      setSession(next);
      setLibrary([]);
      setNotice("");
      setPassword("");
      setEmail("");
      setRegister(false);
      setError("");
      setQuery(""); setBrowseCatalog(false);
      setFilter("ALL");
      setLibraryQuery(""); setLibraryRating("ALL"); setLibrarySort("DEFAULT"); setLibraryProgress("ALL");
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
      if (e instanceof ApiError && e.code === "email_verification_required") {
        setLibrary([]);
        setSession((current) =>
          current
            ? { ...current, user: { ...current.user, emailVerified: false } }
            : current,
        );
      }
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
  useEffect(() => {
    const revision = ++catalogSearchRevision.current;
    setSearchedCatalog(null); setCatalogSearchError(""); setCatalogSearchBusy(false); setCatalogResultLimit(8); setCatalogPage({ trail: [] }); retryPage.current=null;
    if (!showCatalogResults || mode === null) return;
    setCatalogSearchBusy(true);
    const timer = setTimeout(() => {
      const params = new URLSearchParams({ q: query.trim(), limit: "100", cursor: "start" });
      for (const [key, value] of Object.entries(catalogFilters)) if (value) params.set(key, value);
      client.request<{ items: Anime[]; nextCursor?: string }>("/anime?" + params.toString())
        .then(result => { if (revision === catalogSearchRevision.current) {setSearchedCatalog(result.items ?? []);setCatalogPage({ trail: ["start"], next: result.nextCursor });} })
        .catch(error => { if (revision === catalogSearchRevision.current) { setCatalogSearchError("Could not search catalog. Try again."); handleError(error); } })
        .finally(() => { if (revision === catalogSearchRevision.current) setCatalogSearchBusy(false); });
    }, 250);
    return () => { clearTimeout(timer); catalogSearchRevision.current++; };
  }, [query, mode, client, handleError, catalogFilters, showCatalogResults]);
  async function catalogPageRequest(trail: string[]) {
    const cursor = trail.at(-1);
    if (catalogSearchBusy || !cursor) return;
    const revision = ++catalogSearchRevision.current;
    setCatalogSearchBusy(true);
    setCatalogSearchError("");
    retryPage.current = trail;
    const params = new URLSearchParams({ q: query.trim(), limit: "100", cursor });
    for (const [key, value] of Object.entries(catalogFilters)) if (value) params.set(key, value);
    try {
      const result = await client.request<{ items: Anime[]; nextCursor?: string }>("/anime?" + params);
      if (revision !== catalogSearchRevision.current) return;
      setSearchedCatalog(result.items ?? []);
      setCatalogPage({ trail, next: result.nextCursor });
      setCatalogResultLimit(8);
      retryPage.current = null;
    } catch (error) {
      if (revision === catalogSearchRevision.current) {
        setCatalogSearchError("Could not search catalog. Try again.");
        handleError(error);
      }
    } finally {
      if (revision === catalogSearchRevision.current) setCatalogSearchBusy(false);
    }
  }
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
    const revision = ++refreshRevision.current;
    setLoading(true);
    try {
      const [c, l] = await Promise.all([
        client.request<{ items: Anime[] }>("/anime"),
        mode === "local" || session?.user.emailVerified
          ? client.request<{ items: Entry[] }>("/tracker")
          : Promise.resolve({ items: [] as Entry[] }),
      ]);
      if (revision !== refreshRevision.current) return;
      setCatalog(c.items ?? []);
      setLibrary(l.items ?? []);
      setError("");
    } catch (e) {
      if (revision === refreshRevision.current) handleError(e);
    } finally {
      if (revision === refreshRevision.current) setLoading(false);
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
  async function mutate(path: string, method: string, body?: object) {
    const revision = accountRevision.current;
    setBusy(true);
    setError("");
    try {
      await client.request(path, {
        method,
        headers: { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      if (revision !== accountRevision.current) return;
      setRemoval(null);
      setQuery(""); setBrowseCatalog(false); setCatalogFilters(emptyCatalogFilters);
      await refresh();
    } catch (e) {
      handleError(e);
    } finally {
      if (revision === accountRevision.current) setBusy(false);
    }
  }
  const filtered = useMemo(() => {
    const titleQuery = libraryQuery.trim().toLowerCase();
    const entries = library.filter(entry =>
      (filter === "ALL" || entry.watchStatus === filter) &&
      entry.title.toLowerCase().includes(titleQuery) &&
      (libraryProgress === "ALL" || (libraryProgress === "EXACT" ? entry.exactProgress === true : entry.exactProgress !== true)) &&
      (libraryRating === "ALL" ||
        (libraryRating === "UNRATED" ? entry.rating == null :
          libraryRating === "RATED" ? entry.rating != null :
            entry.rating === Number(libraryRating))));
    if (librarySort === "DEFAULT") return entries;
    return entries.sort((a, b) => {
      if (librarySort === "RATING_HIGH" || librarySort === "RATING_LOW") {
        if (a.rating == null && b.rating != null) return 1;
        if (b.rating == null && a.rating != null) return -1;
        if (a.rating != null && b.rating != null && a.rating !== b.rating)
          return librarySort === "RATING_HIGH" ? b.rating - a.rating : a.rating - b.rating;
      }
      if (librarySort === "EPISODES" && a.episodesWatched !== b.episodesWatched)
        return b.episodesWatched - a.episodesWatched;
      return a.title.localeCompare(b.title, "en") || a.id.localeCompare(b.id, "en");
    });
  }, [library, filter, libraryQuery, libraryRating, librarySort, libraryProgress]);
  const matchingCatalog = useMemo(
    () =>
      (searchedCatalog ?? (hasCatalogFilters || browseCatalog ? [] : catalog))
        .filter(
          (x) =>
            (searchedCatalog !== null || x.title.toLowerCase().includes(query.toLowerCase())) &&
            !library.some((e) => e.animeId === x.id),
        ),
    [catalog, searchedCatalog, library, query, hasCatalogFilters, browseCatalog],
  );
  const results = matchingCatalog.slice(0, catalogResultLimit);
  async function add(a: Anime) {
    await mutate("/tracker", "POST", {
      animeId: a.id,
      watchStatus: "PLANNING",
      episodesWatched: 0,
    });
  }
  async function patch(e: Entry, body: object) {
    await mutate("/tracker/" + e.id, "PATCH", { ...body, ...(mode === "postgres" && e.watchCycle ? { expectedCycle: e.watchCycle } : {}) });
  }
  async function rate(e: Entry, score: number) {
    await mutate("/ratings/" + e.animeId, "PUT", { score });
  }
  async function uploadWatchList(preview: boolean) {
    if (busy || !watchListFile) return;
    setWatchListPreview(null);
    if (watchListFile.size > 16 * 1024 * 1024) {
      setError("Choose a watch-list file of 16 MiB or less.");
      return;
    }
    const account = accountRevision.current;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const body = await watchListFile.text();
      if (account !== accountRevision.current) return;
      const result = await client.request<{ added: number; skipped: number }>(preview ? "/tracker/import/preview" : "/tracker/import", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
      });
      if (account !== accountRevision.current) return;
      if (preview) {
        setWatchListPreview(result);
        return;
      }
      setWatchListFile(null);
      setShowWatchListImport(false);
      await refresh();
      if (account === accountRevision.current) {
        setNotice(`Watch list imported. Added ${result.added}. Skipped ${result.skipped} already in your library.`);
      }
    } catch (error) {
      if (account !== accountRevision.current) return;
      if (error instanceof ApiError && error.status === 400) {
        setError("Choose a valid Anime Intelligence watch-list export.");
      } else if (error instanceof ApiError && error.status === 422) {
        setError("Some titles or episode selections do not match this catalog.");
      } else {
        handleError(error);
      }
    } finally {
      if (account === accountRevision.current) setBusy(false);
    }
  }
  async function downloadWatchList() {
    if (busy) return;
    const account = accountRevision.current;
    setBusy(true);
    setError("");
    try {
      const data = await client.request<unknown>("/tracker/export");
      if (account === accountRevision.current) {
        downloadJSON("anime-intelligence-watch-list.json", data);
      }
    } catch (error) {
      if (account === accountRevision.current) handleError(error);
    } finally {
      if (account === accountRevision.current) setBusy(false);
    }
  }
  async function applyBulkEdit(action: "status" | "rating") {
    const value = action === "status" ? bulkStatus : bulkRating;
    if (busy || !bulkEntries?.length || !value) return;
    const change = action === "status" ? { watchStatus: value } : { score: Number(value) };
    const account = accountRevision.current;
    setBusy(true);
    setError("");
    try {
      await client.request("/tracker/bulk-" + action, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...change, entries: bulkEntries }),
      });
      if (account !== accountRevision.current) return;
      setBulkEntries(null);
      setBulkStatus("");
      setBulkRating("");
      await refresh();
    } catch (error) {
      if (account !== accountRevision.current) return;
      if (error instanceof ApiError && (error.status === 409 || error.status === 404)) {
        setBulkEntries([]);
        await refresh();
        if (account === accountRevision.current) {
          setError("The selected entries changed. Select them again and retry.");
        }
      } else {
        handleError(error);
      }
    } finally {
      if (account === accountRevision.current) setBusy(false);
    }
  }
  const canEdit = mode === "local" || !!session?.user.emailVerified;
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
      {session && (
        <AccountManager
          key={`account:${session.user.id}`}
          client={client}
          onDeleted={() => {
            changeSession(null);
            setNotice("Account deleted. All sessions have been signed out.");
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
      {session && !session.user.emailVerified && (
        <section
          aria-label="Email verification required"
          className="mt-4 rounded-2xl border border-amber-300/30 p-5 text-sm text-amber-100"
        >
          <h2 className="font-semibold">
            Verify your email to unlock your library
          </h2>
          <p className="mt-2">
            {identityAvailable
              ? "Request a verification email above and confirm the link. Your session stays signed in while you verify."
              : "Email delivery is currently unavailable on this server. Library access stays locked until you can verify your email."}
          </p>
          <p className="mt-2">
            You can still manage sessions, change your password, download your
            data or delete your account.
          </p>
        </section>
      )}
      {mode === "postgres" && session?.user.emailVerified && (
        <ReleaseAlerts key={`release-alerts:${session.user.id}`} client={client} library={library} onError={handleError} />
      )}
      {mode === "postgres" && session?.user.emailVerified && <ReleaseCalendar key={`calendar:${session.user.id}`} client={client} onError={handleError} />}
      {mode === "postgres" && session?.user.emailVerified && <PersonalMatches key={`matches:${session.user.id}`} client={client} onError={handleError} />}
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
        <button type="button" aria-pressed={browseCatalog} onClick={() => setBrowseCatalog(current => !current)} className="mt-3 text-sm text-violet-200 underline">
          Browse catalog
        </button>
        <CatalogFilters key={"catalog-filters:"+(session?.user.id??"public")} client={client} values={catalogFilters} onApply={values => { catalogSearchRevision.current++; setSearchedCatalog(null); setCatalogFilters({ ...values }); }} />
        {showCatalogResults && catalogSearchBusy && <p role="status" className="mt-3 text-sm text-slate-400">Searching catalog…</p>}
        {catalogSearchError && <p role="alert" className="mt-3 text-sm text-rose-200">{catalogSearchError} <button onClick={() => retryPage.current!==null?void catalogPageRequest(retryPage.current):setCatalogFilters(current => ({ ...current }))} className="underline">Retry catalog search</button></p>}
        {showCatalogResults && searchedCatalog !== null && !catalogSearchBusy && results.length === 0 && <p className="mt-3 text-sm text-slate-400">No matching anime outside your library.</p>}
        {showCatalogResults && (
          <div className="mt-3 space-y-2">
            {results.map((a) => (
              <div
                key={a.id}
                className="rounded-xl bg-white/5 p-3"
              >
                <div className="flex items-center justify-between gap-3">
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
                <CatalogDetails key={`search-catalog:${session?.user.id ?? "public"}:${a.id}`} client={client} id={a.id} title={a.title} onError={handleError} />
              </div>
            ))}
            {matchingCatalog.length > results.length && <button onClick={() => setCatalogResultLimit(current => current + 8)} className="text-sm text-violet-200 underline">Show more catalog matches</button>}
            {searchedCatalog!==null&&<div className="flex gap-4">
              {catalogPage.trail.length > 1 && <button disabled={catalogSearchBusy} onClick={() => void catalogPageRequest(catalogPage.trail.slice(0, -1))} className="text-sm underline">Previous catalog page</button>}
              {catalogPage.next && <button disabled={catalogSearchBusy} onClick={() => { if (catalogPage.next) void catalogPageRequest([...catalogPage.trail, catalogPage.next]); }} className="text-sm underline">Next catalog page</button>}
            </div>}
          </div>
        )}
      </section>
      <section aria-label="My Library" className="mt-10">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <h2 className="text-2xl font-semibold">My Library</h2>
          {mode === "postgres" && canEdit && (
            <button type="button" disabled={busy} onClick={() => void downloadWatchList()} className="text-sm text-violet-200 underline disabled:opacity-50">
              Download watch list
            </button>
          )}
          {mode === "postgres" && canEdit && (
            <button type="button" disabled={busy} aria-expanded={showWatchListImport} onClick={() => {
              setShowWatchListImport(current => !current);
              setWatchListFile(null);
              setWatchListPreview(null);
            }} className="text-sm text-violet-200 underline disabled:opacity-50">
              {showWatchListImport ? "Cancel watch-list import" : "Import watch list"}
            </button>
          )}
          <button
            onClick={() => void refresh()}
            className="text-sm text-slate-400 underline"
          >
            Refresh
          </button>
        </div>
        {mode === "postgres" && canEdit && showWatchListImport && (
          <form onSubmit={event => { event.preventDefault(); void uploadWatchList(false); }} className="mt-4 flex flex-wrap items-end gap-3 text-sm">
            <label>
              Anime Intelligence watch-list JSON
              <input type="file" accept=".json,application/json" disabled={busy} required
                onChange={event => {
                  setWatchListFile(event.target.files?.[0] ?? null);
                  setWatchListPreview(null);
                }}
                className="mt-1 block" />
            </label>
            <button type="button" disabled={busy || !watchListFile} onClick={() => void uploadWatchList(true)}
              className="text-violet-200 underline disabled:opacity-50">
              Preview import
            </button>
            {watchListPreview && (
              <p role="status">Would add {watchListPreview.added}. Would skip {watchListPreview.skipped} already in your library.</p>
            )}
            <button type="submit" disabled={busy || !watchListFile} className="rounded-lg bg-violet-300 px-3 py-2 text-slate-950 disabled:opacity-50">
              Import new titles
            </button>
          </form>
        )}
        <div className="mt-4 flex flex-wrap gap-2">
          {statuses.map((s) => (
            <button
              key={s}
              aria-pressed={filter === s}
              onClick={() => setFilter(s)}
              className={`rounded-full px-4 py-2 text-xs ${filter === s ? "bg-violet-300 text-slate-950" : "border border-white/10 text-slate-400"}`}
            >
              {labels[s]}
            </button>
          ))}
        </div>
        {canEdit && <div className="mt-4 space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="text-sm text-slate-300">Search your library
              <input aria-label="Search your library" type="search" maxLength={2048} value={libraryQuery}
                onChange={event => setLibraryQuery(event.target.value)}
                className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
            </label>
            <label className="text-sm text-slate-300">Library rating filter
              <select aria-label="Library rating filter" value={libraryRating} onChange={event => setLibraryRating(event.target.value)}
                className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-900 p-2">
                <option value="ALL">Any rating</option><option value="RATED">Rated</option><option value="UNRATED">Not rated</option>
                {[1,2,3,4,5].map(score => <option key={score} value={String(score)}>{score} / 5</option>)}
              </select>
            </label>
            <label className="text-sm text-slate-300">Library sort order
              <select aria-label="Library sort order" value={librarySort} onChange={event => setLibrarySort(event.target.value)}
                className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-900 p-2">
                <option value="DEFAULT">Default order</option><option value="TITLE">Title A–Z</option>
                <option value="RATING_HIGH">Highest rating</option><option value="RATING_LOW">Lowest rating</option>
                <option value="EPISODES">Most episodes watched</option>
              </select>
            </label>
          </div>
          <label className="block text-sm text-slate-300">Library progress mode
            <select aria-label="Library progress mode" value={libraryProgress} onChange={event => setLibraryProgress(event.target.value)}
              className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-900 p-2 sm:w-auto">
              <option value="ALL">Any progress mode</option>
              <option value="EXACT">Exact episode selections</option>
              <option value="COUNT">Episode count</option>
            </select>
          </label>
          <button onClick={() => {setFilter("ALL");setLibraryQuery("");setLibraryRating("ALL");setLibrarySort("DEFAULT");setLibraryProgress("ALL");}}
            className="text-sm text-violet-200 underline">Clear library filters</button>
          {!loading && <p role="status" className="text-sm text-slate-400">Showing {filtered.length} of {library.length} library titles.</p>}
          {!loading && filtered.length === 0 && <p className="text-sm text-slate-400">{library.length ? "No library titles match these filters." : "Your library is empty. Add anime from the catalog above."}</p>}
        </div>}
        {mode === "postgres" && canEdit && library.length > 0 && (
          <div className="mt-4 flex flex-wrap items-center gap-3 text-sm">
            <button type="button" disabled={busy} className="underline" onClick={() => {
              setBulkEntries(bulkEntries === null ? [] : null);
              setBulkStatus("");
              setBulkRating("");
            }}>
              {bulkEntries === null ? "Edit multiple entries" : "Cancel bulk edit"}
            </button>
            {bulkEntries !== null && (
              <>
                <span role="status">{bulkEntries.length} selected (maximum {maxBulkStatusEntries})</span>
                <select aria-label="Bulk watch status" disabled={busy} value={bulkStatus} onChange={event => setBulkStatus(event.target.value)}
                  className="rounded-lg border border-white/10 bg-slate-900 p-2">
                  <option value="">Choose status</option>
                  {statuses.slice(1).map(status => <option key={status} value={status}>{labels[status]}</option>)}
                </select>
                <button type="button" disabled={busy || bulkEntries.length === 0 || !bulkStatus}
                  onClick={() => void applyBulkEdit("status")} className="rounded-lg bg-violet-300 px-3 py-2 text-slate-950 disabled:opacity-50">
                  Apply status to selected
                </button>
                <select aria-label="Bulk series rating" disabled={busy} value={bulkRating}
                  onChange={event => setBulkRating(event.target.value)}
                  className="rounded-lg border border-white/10 bg-slate-900 p-2">
                  <option value="">Choose series rating</option>
                  {[1, 2, 3, 4, 5].map(score => <option key={score} value={score}>{score} / 5</option>)}
                </select>
                <button type="button" disabled={busy || bulkEntries.length === 0 || !bulkRating}
                  onClick={() => void applyBulkEdit("rating")} className="rounded-lg bg-violet-300 px-3 py-2 text-slate-950 disabled:opacity-50">
                  Apply rating to selected
                </button>
              </>
            )}
          </div>
        )}
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
            {session
              ? "Verify your email to add anime and view your private library."
              : "Sign in to add anime and view your private library."}
          </p>
        )}
        {loading ? (
          <p className="mt-6 text-slate-500">Loading…</p>
        ) : (
          <div className="mt-5 grid gap-3 md:grid-cols-2">
            {filtered.map((e) => (
              <article
                key={`${session?.user.id ?? "local"}:${e.id}:${e.watchCycle ?? 1}`}
                className="rounded-2xl border border-white/10 bg-white/[.03] p-5"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h3 className="font-semibold">{e.title}</h3>
                    {mode === "postgres" && canEdit && bulkEntries !== null && (
                      <label className="mt-2 flex items-center gap-2 text-xs text-slate-300">
                        <input type="checkbox" aria-label={"Select " + e.title + " for bulk edit"}
                          checked={bulkEntries.some(target => target.id === e.id)}
                          disabled={busy || (bulkEntries.length >= maxBulkStatusEntries && !bulkEntries.some(target => target.id === e.id))}
                          onChange={event => {
                            const checked = event.target.checked;
                            setBulkEntries(current => {
                              if (!current) return current;
                              if (!checked) return current.filter(target => target.id !== e.id);
                              if (current.length >= maxBulkStatusEntries || current.some(target => target.id === e.id)) return current;
                              return [...current, { id: e.id, expectedCycle: e.watchCycle ?? 1 }];
                            });
                          }} />
                        Select for bulk edit
                      </label>
                    )}
                    {mode === "postgres" && <p className="mt-1 text-xs text-slate-500">Viewing pass {e.watchCycle ?? 1}</p>}
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
                    disabled={busy || !!e.exactProgress}
                    title={e.exactProgress ? "Use Edit watched episodes to preserve exact selections." : undefined}
                    onClick={() =>
                      void patch(e, { episodesWatched: e.episodesWatched + 1 })
                    }
                    className="rounded-lg border border-white/10 px-3 py-2 text-xs"
                  >
                    +1 episode
                  </button>
                </div>
                <CatalogDetails key={`catalog:${session?.user.id ?? "public"}:${e.animeId}`} client={client} id={e.animeId} title={e.title} onError={handleError} />
                {mode === "postgres" && session?.user.emailVerified && (
                  <EpisodeEditor key={`episodes:${session.user.id}:${e.id}`} client={client} id={e.id} title={e.title} disabled={busy || !canEdit} onSaved={refresh} onError={handleError} />
                )}
                {mode === "postgres" && session?.user.emailVerified && (
                  <RewatchManager client={client} id={e.id} title={e.title} completed={e.watchStatus === "COMPLETED"} disabled={busy || !canEdit} onSaved={refresh} onError={handleError} />
                )}
                {mode === "postgres" && session?.user.emailVerified && (
                  <NotesEditor client={client} id={e.id} title={e.title} disabled={busy || !canEdit} onSaved={refresh} onError={handleError} />
                )}
                {e.exactProgress && <p className="mt-2 text-xs text-slate-400">Exact episodes tracked; use the episode editor to change your selection.</p>}
                <div className="mt-4 border-t border-white/10 pt-3">
                  {removal === e.id ? (
                    <div role="group" aria-label={"Remove " + e.title}>
                      <p className="text-sm text-slate-300">
                        Remove {e.title} from your library? This also deletes its
                        ratings, progress, notes and archived viewing passes. Adding it again starts fresh.
                      </p>
                      <div className="mt-3 flex gap-3">
                        <button disabled={busy || !canEdit}
                          onClick={() => void mutate("/tracker/" + e.id, "DELETE")}
                          className="rounded-lg border border-rose-400/30 px-3 py-2 text-xs text-rose-200 disabled:opacity-50">
                          {busy ? "Please wait…" : "Confirm removal"}
                        </button>
                        <button disabled={busy} onClick={() => setRemoval(null)}
                          className="text-xs text-slate-300 underline">Cancel</button>
                      </div>
                    </div>
                  ) : (
                    <button disabled={busy || !canEdit} onClick={() => setRemoval(e.id)}
                      aria-label={"Remove " + e.title}
                      className="text-xs text-rose-300 underline disabled:opacity-50">
                      Remove from library
                    </button>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
      <footer className="mt-14 border-t border-white/10 pt-5 text-xs text-slate-600">
        Anime Intelligence · Development foundation ·{" "}
        {mode === "postgres" ? "Private account library" : "Local demo profile"}
        <p className="mt-2"><ProviderCredit provider="tvmaze" /></p>
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
