"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { ApiClient } from "../lib/api";

type Month = { month: string; archived: number; rated: number; averageRating: number | null };
type LibraryAnswer = {
  topicMatched: boolean;
  fromMonth?: string;
  throughMonth?: string;
  archivedPasses?: number;
  archiveRatings?: { rated: number; average: number | null };
};

const monthLabel = new Intl.DateTimeFormat(undefined, { year: "numeric", month: "short", timeZone: "UTC" });
const archiveMonth = (month: string) => monthLabel.format(new Date(month + "-01T00:00:00Z"));

export default function MonthlyViewingHistory({ client, onError }: { client: ApiClient; onError: (error: unknown) => void }) {
  const [months, setMonths] = useState<Month[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [question, setQuestion] = useState("");
  const [answer, setAnswer] = useState<LibraryAnswer | null>(null);
  const [asking, setAsking] = useState(false);
  const [answerFailed, setAnswerFailed] = useState(false);
  const revision = useRef(0);
  const answerRevision = useRef(0);
  useEffect(() => () => { revision.current++; answerRevision.current++; }, []);

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

  async function ask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!question.trim() || asking) return;
    const current = ++answerRevision.current;
    setAsking(true);
    setAnswer(null);
    setAnswerFailed(false);
    try {
      const response = await client.request<LibraryAnswer>("/analysis/library", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ question: question.trim() }),
      });
      if (current === answerRevision.current) setAnswer(response);
    } catch (error) {
      if (current === answerRevision.current) {
        setAnswerFailed(true);
        onError(error);
      }
    } finally {
      if (current === answerRevision.current) setAsking(false);
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
                      <th scope="row" className="py-1 pr-2 font-normal">{archiveMonth(month.month)}</th>
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
      <form onSubmit={ask} className="mt-4 space-y-2">
        <label className="block">
          Ask about archived passes or series ratings
          <input value={question} maxLength={400} disabled={asking} onChange={event => setQuestion(event.target.value)}
            className="mt-1 block w-full rounded-lg border border-white/10 bg-slate-950 p-2" />
        </label>
        <button type="submit" disabled={asking || !question.trim()} className="text-violet-200 underline disabled:opacity-50">
          {asking ? "Looking up archive facts…" : "Show archive facts"}
        </button>
      </form>
      {answerFailed && <p role="alert" className="mt-2 text-rose-200">Could not load archive facts. Try again.</p>}
      {answer && (
        <div role="status" className="mt-2 space-y-1 text-slate-300">
          {answer.topicMatched && answer.fromMonth && answer.throughMonth ? (
            <>
              <p>Archive months {archiveMonth(answer.fromMonth)}–{archiveMonth(answer.throughMonth)} (UTC)</p>
              {answer.archivedPasses != null && <p>{answer.archivedPasses} previous completed {answer.archivedPasses === 1 ? "pass" : "passes"} archived</p>}
              {answer.archiveRatings && <p>{answer.archiveRatings.average == null
                ? "No archived passes in this period had a series rating."
                : `Average series rating ${answer.archiveRatings.average.toFixed(1)} / 5 across ${answer.archiveRatings.rated} rated ${answer.archiveRatings.rated === 1 ? "pass" : "passes"}`}</p>}
            </>
          ) : <p>Ask about previous viewing passes or their archived series ratings.</p>}
        </div>
      )}
    </section>
  );
}
