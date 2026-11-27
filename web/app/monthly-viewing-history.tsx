"use client";

import { useEffect, useRef, useState } from "react";
import { ApiClient } from "../lib/api";

type Month = { month: string; archived: number; rated: number; averageRating: number | null };
const monthLabel = new Intl.DateTimeFormat(undefined, { year: "numeric", month: "short", timeZone: "UTC" });

export default function MonthlyViewingHistory({ client, onError }: { client: ApiClient; onError: (error: unknown) => void }) {
  const [months, setMonths] = useState<Month[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const revision = useRef(0);
  useEffect(() => () => { revision.current++; }, []);

  async function load() {
    const current = ++revision.current;
    setBusy(true);
    setFailed(false);
    try {
      const result = await client.request<{ months: Month[] }>("/tracker/viewing-history-summary");
      if (current === revision.current) setMonths(result.months);
    } catch (error) {
      if (current === revision.current) {
        setFailed(true);
        onError(error);
      }
    } finally {
      if (current === revision.current) setBusy(false);
    }
  }

  return (
    <section aria-labelledby="monthly-viewing-history-title" className="mt-5 border-t border-white/10 pt-4 text-sm">
      <h3 id="monthly-viewing-history-title" className="font-medium">Monthly viewing history</h3>
      <button type="button" disabled={busy} onClick={() => void load()} className="mt-2 text-violet-200 underline disabled:opacity-50">
        {busy ? "Loading monthly viewing history…" : months ? "Refresh monthly viewing history" : "Load monthly viewing history"}
      </button>
      {failed && <p role="alert" className="mt-2 text-rose-200">Could not load monthly viewing history. Try again.</p>}
      {months && (
        <>
          <p className="mt-2 text-slate-300">Previous completed passes by archive month (UTC)</p>
          {months.some(month => month.archived > 0) ? (
            <div className="mt-2 overflow-x-auto">
              <table className="w-full border-collapse text-left text-slate-300">
                <caption className="sr-only">Completed viewing passes by archive month</caption>
                <thead>
                  <tr className="border-b border-white/10">
                    <th scope="col" className="py-1 pr-2">Month</th>
                    <th scope="col" className="px-2 py-1 text-right">Archived</th>
                    <th scope="col" className="px-2 py-1 text-right">Rated</th>
                    <th scope="col" className="py-1 pl-2 text-right">Avg. series rating</th>
                  </tr>
                </thead>
                <tbody>
                  {months.map(month => (
                    <tr key={month.month} className="border-b border-white/5">
                      <th scope="row" className="py-1 pr-2 font-normal">{monthLabel.format(new Date(month.month + "-01T00:00:00Z"))}</th>
                      <td className="px-2 py-1 text-right tabular-nums">{month.archived}</td>
                      <td className="px-2 py-1 text-right tabular-nums">{month.rated}</td>
                      <td className="py-1 pl-2 text-right tabular-nums">{month.averageRating == null ? "—" : month.averageRating.toFixed(1) + " / 5"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : <p className="mt-1 text-slate-300">No viewing passes were archived in the last twelve months.</p>}
        </>
      )}
    </section>
  );
}
