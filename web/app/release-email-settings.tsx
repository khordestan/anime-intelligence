"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Consent = { enabledSince: string | null };
const path = "/releases/email-consent";

export default function ReleaseEmailSettings({ client }: { client: ApiClient }) {
  const [consent, setConsent] = useState<Consent | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const revision = useRef(0);
  useEffect(() => () => { revision.current++; }, []);

  async function request(enabled?: boolean) {
    const current = ++revision.current;
    setBusy(true);
    setError("");
    try {
      const next = await client.request<Consent>(path, enabled === undefined ? undefined : {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      if (current === revision.current) setConsent(next);
    } catch {
      if (current === revision.current) setError("Could not update release email settings. Try again.");
    } finally {
      if (current === revision.current) setBusy(false);
    }
  }

  return (
    <div className="mt-4 space-y-1 text-sm">
      {consent === null ? (
        <button type="button" disabled={busy} onClick={() => void request()} className="underline disabled:opacity-50">
          Release email settings
        </button>
      ) : (
        <>
          <p>Release email preference is {consent.enabledSince ? "on" : "off"}.</p>
          <p className="text-slate-400">When email delivery is configured, get updates for confirmed releases and linked-season announcements.</p>
          <button type="button" disabled={busy} onClick={() => void request(!consent.enabledSince)} className="underline disabled:opacity-50">
            {consent.enabledSince ? "Turn off release email" : "Turn on release email"}
          </button>
        </>
      )}
      {error && <p role="status" className="text-rose-200">{error}</p>}
    </div>
  );
}
