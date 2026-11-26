import { test, expect, type Page } from "@playwright/test";

async function mockMatches(page: Page, hold = false) {
  let issued = 0;
  const planned = new Set<string>();
  const feedback = new Map<string, number>();
  const dismissed = new Set<string>();
  const preferences = new Map<string, number | null>();
  const calls: { path: string; query: string; token: string; body: unknown }[] = [];
  let release = () => {};
  let blocked = false;
  let failLimit = false;
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace("Bearer ", "");
    const headers = { "Access-Control-Allow-Origin": "http://127.0.0.1:3000", "Access-Control-Allow-Headers": "Authorization, Content-Type", "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS" };
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    calls.push({ path, query: url.search, token, body: request.postData() ? request.postDataJSON() : undefined });
    if (path === "") return respond({ authenticationRequired: true });
    if (path === "/anime" || path === "/tracker") return respond({ items: [] });
    if (path === "/auth/login") {
      const id = String(++issued), token = id.padStart(43, "a");
      preferences.set(token, hold && issued === 1 ? 1 : null);
      return respond({ token, user: { id, email: request.postDataJSON().email, displayName: "", emailVerified: true }, expiresAt: new Date(Date.now() + 86_400_000).toISOString() });
    }
    if (!preferences.has(token)) return respond({ error: "unauthorized" }, 401);
    if (path === "/auth/logout") return route.fulfill({ status: 204, headers });
    if (path === "/preferences") return respond({ items: [{ key: "synthetic.mystery", name: "Synthetic mystery", category: "Synthetic story", weight: preferences.get(token) }] });
    if (path === "/preferences/synthetic.mystery") {
      preferences.set(token, request.method() === "DELETE" ? null : request.postDataJSON().weight);
      return route.fulfill({ status: 204, headers });
    }
    if (path === "/recommendations/dismissals") return respond({ items: dismissed.has(token) ? [{ animeId: "synthetic-match", title: "Synthetic account match" }] : [] });
    if (path === "/recommendations/dismissals/synthetic-match") {
      if (request.method() === "DELETE") dismissed.delete(token);
      else dismissed.add(token);
      return route.fulfill({ status: 204, headers });
    }
    if (path === "/tracker/import") {
      planned.add(token);
      return respond({ added: 1, skipped: 0 });
    }
    if (path === "/recommendations/discovery/feedback") {
      return respond({ items: feedback.has(token) ? [{ animeId: "synthetic-match", title: "Synthetic account match", direction: feedback.get(token) }] : [] });
    }
    if (path === "/recommendations/discovery/feedback/synthetic-match") {
      if (request.method() === "DELETE") feedback.delete(token);
      else feedback.set(token, request.postDataJSON().direction);
      return route.fulfill({ status: 204, headers });
    }
    if (path === "/recommendations/discovery") {
      const featureKey = url.searchParams.get("featureKey");
      const after = url.searchParams.get("after");
      if (planned.has(token) || dismissed.has(token)) return respond({ items: [] });
      if (url.searchParams.get("unfamiliar") === "true") {
        if (after) {
          expect(after).toBe("synthetic-match");
          expect(url.searchParams.get("afterCount")).toBe("1");
          expect(url.searchParams.get("afterUnfamiliar")).toBe("true");
          return respond({ items: [{ animeId: "synthetic-novel", title: "Synthetic unfamiliar sequel", evidenceCount: 2, unfamiliarCount: 1 }] });
        }
        return respond({
          items: [{ animeId: "synthetic-match", title: "Synthetic account match", evidenceCount: 2, unfamiliarCount: 1 }],
          nextCursor: { animeId: "synthetic-match", evidenceCount: 1, unfamiliar: true },
        });
      }
      if (after) {
        if (after !== "synthetic-match" || featureKey !== "synthetic.mystery" ||
          url.searchParams.get("afterFeature") !== featureKey || url.searchParams.get("afterCount") !== "1") {
          return respond({ error: "invalid_discovery_cursor" }, 400);
        }
        return respond({ items: [{ animeId: "synthetic-next", title: "Synthetic mystery sequel", evidenceCount: 1 }] });
      }
      return respond({
        items: [{ animeId: "synthetic-match", title: "Synthetic account match", evidenceCount: 1, supportedFeatures: ["Synthetic mystery"], featuredEvidence: [{ key: "synthetic.mystery", name: "Synthetic mystery" }], feedbackDirection: feedback.get(token) }],
        nextCursor: featureKey ? { animeId: "synthetic-match", evidenceCount: 1, featureKey } : undefined,
      });
    }
    if (path === "/recommendations") {
      if (failLimit && url.searchParams.has("limit")) {
        failLimit = false;
        return respond({ error: "synthetic_failed_request" }, 500);
      }
      const items = preferences.get(token) === null || dismissed.has(token) ? [] : [{ animeId: "synthetic-match", title: "Synthetic account match", score: 90, evidence: [{ featureKey: "synthetic.mystery", direction: "match", strength: 1, impact: 1 }] }];
      if (hold && token.endsWith("1")) { blocked = true; await new Promise<void>(resolve => { release = resolve; }); }
      try { return await respond({ items, profile: feedback.has(token) ? [{ key: "synthetic.mystery", feedback: { weight: feedback.get(token), confidence: 0.2, feedbackCount: 1 } }] : items.length ? [{ key: "synthetic.mystery", declared: { weight: preferences.get(token), confidence: 1 }, inferred: { weight: 0.5, confidence: 0.25 } }] : [] }); } catch { return; }
    }
    return respond({}, 404);
  });
  return { calls, failNextLimit: () => { failLimit = true; }, blocked: () => blocked, release: () => release() };
}
async function login(page: Page, email: string) {
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill("synthetic-match-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
}
test("preferences save/reset and unsupported evidence remain explicit", async ({ page }) => {
  const api = await mockMatches(page);
  await page.goto("/");
  await login(page, "match-browser@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  await expect(panel.getByText(/No supported matches yet/)).toBeVisible();
  await panel.getByLabel("Preference Synthetic mystery", { exact: true }).fill("1");
  await panel.getByRole("button", { name: "Save preference Synthetic mystery", exact: true }).click();
  await expect(panel.getByText("Synthetic account match", { exact: false })).toBeVisible();
  await expect(panel.getByText("Rating-derived signal: prefers this feature · strength 0.25", { exact: true })).toBeVisible();
  expect(api.calls.some(call => call.path === "/preferences/synthetic.mystery" && (call.body as { weight?: number } | undefined)?.weight === 1)).toBe(true);
  await panel.getByRole("button", { name: "Reset preference Synthetic mystery", exact: true }).click();
  await expect(panel.getByText(/No supported matches yet/)).toBeVisible();
  await expect(panel.getByLabel("Preference Synthetic mystery", { exact: true })).toHaveValue("");
  expect(await page.evaluate(() => ({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) }))).toEqual({ local: [], session: [] });
});
test("late Personal Match results do not cross accounts", async ({ page }) => {
  const api = await mockMatches(page, true);
  await page.goto("/");
  await login(page, "match-first@example.test");
  await page.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  await expect.poll(api.blocked).toBe(true);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "match-second@example.test");
  api.release();
  await page.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  await expect(page.getByText(/No supported matches yet/)).toBeVisible();
  await expect(page.getByText("Synthetic account match", { exact: false })).toHaveCount(0);
  await expect(page.getByText(/Rating-derived signal:/)).toHaveCount(0);
});

test("Personal Match can restore a hidden title from the saved list", async ({ page }) => {
  await mockMatches(page);
  await page.goto("/");
  await login(page, "match-restore@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  await panel.getByLabel("Preference Synthetic mystery", { exact: true }).fill("1");
  await panel.getByRole("button", { name: "Save preference Synthetic mystery", exact: true }).click();
  await panel.getByRole("button", { name: "Hide match Synthetic account match", exact: true }).click();
  await expect(panel.getByText("Match hidden.", { exact: true })).toBeVisible();
  await panel.getByRole("button", { name: "Manage hidden matches", exact: true }).click();
  await panel.getByRole("button", { name: "Restore match Synthetic account match", exact: true }).click();
  await expect(panel.getByText("Match restored.", { exact: true })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Hide match Synthetic account match", exact: true })).toBeVisible();
});

test("Discovery works before adding preferences and respects hidden titles", async ({ page }) => {
  await mockMatches(page);
  await page.goto("/");
  await login(page, "discovery@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  await expect(panel.getByText(/No supported matches yet/)).toBeVisible();
  await panel.getByRole("button", { name: "Explore titles", exact: true }).click();
  const discovery = panel.getByRole("region", { name: "Discovery titles" });
  await expect(discovery.getByText("1 supported feature")).toBeVisible();
  const explore = discovery.getByRole("button", { name: "Explore Synthetic mystery" });
  await expect(explore).toBeVisible();
  const filter = discovery.getByLabel("Filter by feature");
  await explore.click();
  await expect(filter).toHaveValue("synthetic.mystery");
  await discovery.getByRole("button", { name: "Load more discovery titles" }).click();
  await expect(discovery.getByText("Synthetic mystery sequel")).toBeVisible();
  await filter.selectOption("");
  await expect(discovery.getByText("Synthetic mystery sequel")).toHaveCount(0);
  const unfamiliar = discovery.getByRole("checkbox", { name: "Explore features missing from my library" });
  await unfamiliar.click();
  await expect(discovery.getByText("1 feature missing from your library")).toBeVisible();
  await discovery.getByRole("button", { name: "Load more discovery titles" }).click();
  await expect(discovery.getByText("Synthetic unfamiliar sequel")).toBeVisible();
  await unfamiliar.click();
  await expect(discovery.getByText("Synthetic unfamiliar sequel")).toHaveCount(0);
  await discovery.getByRole("button", { name: "Hide discovery title Synthetic account match" }).click();
  await expect(discovery.getByText("No eligible discovery titles yet.")).toBeVisible();
});

test("planning a discovery title uses add-only import", async ({ page }) => {
  const api = await mockMatches(page);
  await page.goto("/");
  await login(page, "discovery-planning@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings", exact: true }).click();
  await panel.getByRole("button", { name: "Explore titles", exact: true }).click();
  await panel.getByRole("button", { name: "Plan to watch Synthetic account match" }).click();
  await expect(panel.getByText("Added to your library as Planning.", { exact: true })).toBeVisible();
  expect(api.calls.find(call => call.path === "/tracker/import")?.body).toEqual({
    version: 1, entries: [{ animeId: "synthetic-match", watchStatus: "PLANNING" }],
  });
  await expect(panel.getByText("No eligible discovery titles yet.")).toBeVisible();
});

test("Discovery saves and clears explicit feature feedback for Personal Match", async ({ page }) => {
  const api = await mockMatches(page);
  await page.goto("/");
  await login(page, "discovery-preference@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings" }).click();
  await panel.getByRole("button", { name: "Explore titles" }).click();
  const discovery = panel.getByRole("region", { name: "Discovery titles" });
  await discovery.getByRole("button", { name: "Explore Synthetic mystery" }).click();
  const feedback = discovery.getByRole("group", { name: "Synthetic mystery preference for Personal Match" });
  const prefer = feedback.getByRole("button", { name: "Prefer Synthetic mystery" });
  await prefer.click();
  await expect(prefer).toHaveAttribute("aria-pressed", "true");
  await expect(panel.getByLabel("Preference Synthetic mystery", { exact: true })).toHaveValue("1");
  const avoid = feedback.getByRole("button", { name: "Avoid Synthetic mystery" });
  await avoid.click();
  await expect(avoid).toHaveAttribute("aria-pressed", "true");
  await expect(panel.getByLabel("Preference Synthetic mystery", { exact: true })).toHaveValue("-1");
  await feedback.getByRole("button", { name: "Clear Synthetic mystery preference" }).click();
  await expect(avoid).toHaveAttribute("aria-pressed", "false");
  await expect(panel.getByLabel("Preference Synthetic mystery", { exact: true })).toHaveValue("");
  await expect(discovery.getByLabel("Filter by feature")).toHaveValue("synthetic.mystery");
  expect(api.calls.filter(call => call.path === "/preferences/synthetic.mystery").map(call => call.body))
    .toEqual([{ weight: 1 }, { weight: -1 }, undefined]);
});

test("Personal Match result limits preserve the current result on failure and persist through refresh", async ({ page }) => {
  const api = await mockMatches(page);
  await page.goto("/");
  await login(page, "match-limit@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings" }).click();
  const limit = panel.getByLabel("Personal Match results limit");
  await expect(limit).toBeEnabled();
  await panel.getByLabel("Preference Synthetic mystery", { exact: true }).fill("1");
  await panel.getByRole("button", { name: "Save preference Synthetic mystery", exact: true }).click();
  const shown = panel.getByRole("list", { name: "Personal match results" }).getByText("Synthetic account match");
  await expect(shown).toBeVisible();
  api.failNextLimit();
  await limit.selectOption("50");
  await expect(panel.getByText("Could not change the results limit. Try again.")).toBeVisible();
  await expect(limit).toHaveValue("20");
  await expect(shown).toBeVisible();
  await limit.selectOption("50");
  await expect(limit).toHaveValue("50");
  await panel.getByRole("button", { name: "Refresh Personal Match" }).click();
  await expect(limit).toBeEnabled();
  expect(api.calls.filter(call => call.path === "/recommendations" && call.query === "?limit=50")).toHaveLength(3);
});

test("Discovery title reactions can be managed after a title leaves Discovery", async ({ page }) => {
  await mockMatches(page);
  await page.goto("/");
  await login(page, "title-feedback@example.test");
  const panel = page.getByRole("region", { name: "Personal Match", exact: true });
  await panel.getByRole("button", { name: "Personal Match settings" }).click();
  await panel.getByRole("button", { name: "Explore titles" }).click();
  const more = panel.getByRole("button", { name: "More like this", exact: true });
  await more.click();
  await expect(more).toHaveAttribute("aria-pressed", "true");
  await expect(panel.getByText("Discovery feedback updated.")).toBeVisible();
  await panel.getByText("Signals from Discovery feedback").click();
  await expect(panel.getByText("Synthetic mystery · More · 1 title reaction")).toBeVisible();
  await panel.getByRole("button", { name: "Hide discovery title Synthetic account match" }).click();
  await expect(more).toHaveCount(0);
  await panel.getByRole("button", { name: "Manage title feedback" }).click();
  const saved = panel.getByRole("region", { name: "Saved Discovery title feedback" });
  await saved.getByRole("button", { name: "Clear feedback for Synthetic account match" }).click();
  await expect(saved.getByText("No saved title feedback.")).toBeVisible();
  await expect(panel.getByText("Signals from Discovery feedback")).toHaveCount(0);
});
