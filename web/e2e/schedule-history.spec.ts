import { expect, test } from "@playwright/test";

test("catalog schedule revisions paginate and keep a scheduled time unconfirmed", async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000001";
  const title = "Synthetic schedule";
  let requests = 0;
  await page.route("**/api/v1**", async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    const respond = (json: unknown, status = 200) => route.fulfill({ status, json });
    if (path === "") return respond({ authenticationRequired: false });
    if (path === "/anime") return respond({ items: [] });
    if (path === "/tracker") return respond({ items: [{ id: "entry", animeId: id, title, watchStatus: "WATCHING", episodesWatched: 0 }] });
    if (path === "/anime/" + id + "/details") {
      return respond({
        anime: { id, title, episodeCountKnown: false },
        genres: [], studios: [], tags: [], synonyms: [], relations: [],
        episodes: [{ number: 1, track: "SUB", scheduledAt: "2026-06-12T00:00:00Z", revision: 3 }],
      });
    }
    if (path === "/anime/" + id + "/episodes/1/schedule-history") {
      requests++;
      expect(url.searchParams.get("track")).toBe("SUB");
      if (url.searchParams.get("beforeRevision") === "3") {
        return respond({ items: [{ revision: 2, scheduledAt: "2026-06-10T00:00:00Z", provider: "tvmaze" }], nextBeforeRevision: 2 });
      }
      expect(url.searchParams.get("beforeRevision")).toBe("2");
      if (requests === 2) return respond({ error: "unavailable" }, 500);
      return respond({ items: [{ revision: 1 }] });
    }
    return respond({ error: "not_found" }, 404);
  });
  await page.goto("/");
  await page.getByRole("button", { name: "View catalog details " + title }).click();
  expect(requests).toBe(0);
  await page.getByRole("button", { name: "Schedule history for episode 1 SUB" }).click();
  const history = page.getByRole("region", { name: "Schedule history for episode 1 SUB" });
  await expect(history.getByText(/Revision 2 · Scheduled/)).toBeVisible();
  await expect(history.getByText(/Confirmed/)).toHaveCount(0);
  await expect(history.getByRole("link", { name: "TVmaze", exact: true })).toBeVisible();
  await history.getByRole("button", { name: "Older revisions" }).click();
  await expect(history.getByText(/Could not load schedule history/)).toBeVisible();
  await expect(history.getByText(/Revision 2 · Scheduled/)).toBeVisible();
  await history.getByRole("button", { name: "Retry" }).click();
  await expect(history.getByText("Revision 1 · Time unknown")).toBeVisible();
});
