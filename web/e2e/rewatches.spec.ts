import { test, expect, type Page } from "@playwright/test";

async function mockRewatches(page: Page, hold = false) {
  let issued = 0, attempts = 0, monthlyRequests = 0;
  let cycle = 1, blocked = false;
  let release = () => {};
  const posts: unknown[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request(), url = new URL(request.url());
    const path = url.pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace("Bearer ", "");
    const headers = { "Access-Control-Allow-Origin": "http://127.0.0.1:3000", "Access-Control-Allow-Headers": "Authorization, Content-Type", "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS" };
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: true });
    if (path === "/anime") return respond({ items: [] });
    if (path === "/auth/login") {
      const id = String(++issued);
      return respond({ token: id.padStart(43, "a"), user: { id, email: request.postDataJSON().email, emailVerified: true }, expiresAt: new Date(Date.now() + 86_400_000).toISOString() });
    }
    if (path === "/auth/logout") return route.fulfill({ status: 204, headers });
    if (path === "/tracker") return respond({ items: token.endsWith("1") ? [{ id: "synthetic-entry", animeId: "synthetic-title", title: "Synthetic Rewatch", watchStatus: cycle === 1 ? "COMPLETED" : "WATCHING", episodesWatched: cycle === 1 ? 3 : 0, exactProgress: cycle === 1, totalEpisodes: 12, rating: 5, watchCycle: cycle }] : [] });
    if (path === "/analysis/library") {
      expect(token.endsWith("1")).toBe(true);
      const body = request.postDataJSON();
      if (body.question === "How many passes and what were my ratings?") {
        return respond({ topicMatched: true, fromMonth: "2025-02", throughMonth: "2026-01",
          archivedPasses: 4, archiveRatings: { rated: 2, average: 4.5 } });
      }
      return respond({ topicMatched: false });
    }
    if (path === "/tracker/viewing-history-summary") {
      monthlyRequests++;
      expect(token.endsWith("1")).toBe(true);
      if (hold) { blocked = true; await new Promise<void>(resolve => { release = resolve; }); }
      else if (monthlyRequests === 1) return respond({ error: "synthetic_failure" }, 500);
      try {
        return await respond({ months: [
          { month: "2026-01", archived: 3, rated: 2, averageRating: 4.5 },
          { month: "2025-12", archived: 1, rated: 0, averageRating: null },
        ] });
      } catch { return; }
    }
    if (path === "/tracker/synthetic-entry/rewatches") {
      if (request.method() === "POST") {
        posts.push(request.postDataJSON());
        if (++attempts === 1) return respond({ error: "synthetic_failure" }, 500);
        cycle = 2;
        return respond({ watchCycle: 2 });
      }
      if (hold) { blocked = true; await new Promise<void>(resolve => { release = resolve; }); }
      try { return await respond({ currentCycle: cycle, nextBefore: 0, items: cycle === 1 ? [] : [{ cycle: 1, count: 3, total: 12, mode: "EXACT", numbers: [1,4,7], rating: 5, archivedAt: "2026-01-01T00:00:00Z" }] }); } catch { return; }
    }
    return respond({}, 404);
  });
  return { posts, monthlyRequests: () => monthlyRequests, blocked: () => blocked, release: () => release() };
}
async function login(page: Page, email: string) {
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill("synthetic-rewatch-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
}
test("rewatch requires confirmation, retries safely and retains exact history", async ({ page }) => {
  const api = await mockRewatches(page);
  await page.goto("/");
  await login(page, "rewatch-browser@example.test");
  await page.getByRole("button", { name: "Viewing history Synthetic Rewatch", exact: true }).click();
  const panel = page.getByRole("region", { name: "Viewing history Synthetic Rewatch", exact: true });
  await expect(panel.getByText("No archived viewing passes yet.", { exact: true })).toBeVisible();
  await panel.getByRole("button", { name: "Start rewatch", exact: true }).click();
  expect(api.posts).toHaveLength(0);
  await panel.getByRole("button", { name: "Cancel rewatch", exact: true }).click();
  expect(api.posts).toHaveLength(0);
  await panel.getByRole("button", { name: "Start rewatch", exact: true }).click();
  await panel.getByRole("button", { name: "Confirm start rewatch", exact: true }).click();
  await expect(panel.getByText(/Could not start rewatch/)).toBeVisible();
  await expect(page.getByText("Viewing pass 1", { exact: true })).toBeVisible();
  await panel.getByRole("button", { name: "Refresh viewing history", exact: true }).click();
  await panel.getByRole("button", { name: "Start rewatch", exact: true }).click();
  await panel.getByRole("button", { name: "Confirm start rewatch", exact: true }).click();
  await expect(page.getByText("Viewing pass 2", { exact: true })).toBeVisible();
  expect(api.posts).toEqual([{ expectedCycle: 1 }, { expectedCycle: 1 }]);
  await page.getByRole("button", { name: "Viewing history Synthetic Rewatch", exact: true }).click();
  await expect(panel.getByText("Exact watched episodes: 1, 4, 7", { exact: true })).toBeVisible();
  await expect(panel.getByText("Pass 1 · 3 / 12 episodes · Rating 5", { exact: true })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Start rewatch", exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => ({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) }))).toEqual({ local: [], session: [] });
});
test("late viewing history cannot cross accounts", async ({ page }) => {
  const api = await mockRewatches(page, true);
  await page.goto("/");
  await login(page, "rewatch-first@example.test");
  await page.getByRole("button", { name: "Viewing history Synthetic Rewatch", exact: true }).click();
  await expect.poll(api.blocked).toBe(true);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "rewatch-second@example.test");
  api.release();
  await expect(page.getByRole("region", { name: "Viewing history Synthetic Rewatch", exact: true })).toHaveCount(0);
  await expect(page.getByText("Synthetic Rewatch", { exact: true })).toHaveCount(0);
});

test("monthly viewing history loads explicitly, retries, and renders archive totals", async ({ page }) => {
  const api = await mockRewatches(page);
  await page.goto("/");
  await login(page, "monthly-history@example.test");
  await page.getByText("Library summary", { exact: true }).click();
  const summary = page.getByRole("region", { name: "Monthly viewing history", exact: true });
  await expect(summary.getByRole("button", { name: "Load monthly viewing history" })).toBeVisible();
  expect(api.monthlyRequests()).toBe(0);
  await summary.getByRole("button", { name: "Load monthly viewing history" }).click();
  await expect(summary.getByRole("alert")).toBeVisible();
  await summary.getByRole("button", { name: "Load monthly viewing history" }).click();
  await expect(summary.getByRole("table").getByRole("row").nth(1).getByRole("cell")).toHaveText(["3", "2", "4.5 / 5"]);
  await expect(summary.getByRole("table").getByRole("row").nth(2).getByRole("cell")).toHaveText(["1", "0", "—"]);
  const question = summary.getByLabel("Ask about archived passes or series ratings");
  await question.fill("How many passes and what were my ratings?");
  await summary.getByRole("button", { name: "Show archive facts" }).click();
  await expect(summary.getByText("4 previous completed passes archived", { exact: true })).toBeVisible();
  await expect(summary.getByText("Average series rating 4.5 / 5 across 2 rated passes", { exact: true })).toBeVisible();
  await question.fill("Where can I stream anime?");
  await summary.getByRole("button", { name: "Show archive facts" }).click();
  await expect(summary.getByText("Ask about previous viewing passes or their archived series ratings.", { exact: true })).toBeVisible();
  await expect(summary.getByText("4 previous completed passes archived", { exact: true })).toHaveCount(0);
});

test("late monthly viewing history cannot cross accounts", async ({ page }) => {
  const api = await mockRewatches(page, true);
  await page.goto("/");
  await login(page, "monthly-first@example.test");
  await page.getByText("Library summary", { exact: true }).click();
  await page.getByRole("button", { name: "Load monthly viewing history" }).click();
  await expect.poll(api.blocked).toBe(true);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "monthly-second@example.test");
  api.release();
  await expect(page.getByRole("region", { name: "Monthly viewing history" })).toHaveCount(0);
  await expect(page.getByText("4.5 / 5", { exact: true })).toHaveCount(0);
});
