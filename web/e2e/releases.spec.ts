import { test, expect, type Page } from "@playwright/test";

const anime = "10000000-0000-4000-8000-000000000001";
async function mockReleases(page: Page, holdFirst = false) {
  let issued = 0;
  let release = () => {};
  let blocked = false;
  const calls: { path: string; method: string; token: string; body: unknown }[] = [];
  const follows = new Map<string, { animeId: string; title: string; track: string }[]>();
  const notices = new Map<string, { id: string; title: string; body: string; createdAt: string; readAt?: string }[]>();
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace("Bearer ", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
    };
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    calls.push({ path, method: request.method(), token, body: request.postData() ? request.postDataJSON() : undefined });
    if (path === "") return respond({ authenticationRequired: true, identityDeliveryAvailable: false });
    if (path === "/anime") return respond({ items: [{ id: anime, title: "Synthetic Release Sky", mediaType: "TV" }] });
    if (path === "/auth/login") {
      const id = String(++issued);
      const token = id.padStart(43, "a");
      follows.set(token, []);
      notices.set(token, issued === 1 ? [{ id: "notice-one", title: "New episode released", body: "Private synthetic release for account one", createdAt: new Date().toISOString() }] : []);
      return respond({ token, user: { id, email: request.postDataJSON().email, displayName: "", emailVerified: true }, expiresAt: new Date(Date.now() + 86_400_000).toISOString() });
    }
    if (!follows.has(token)) return respond({ error: "unauthorized" }, 401);
    if (path === "/auth/logout") return route.fulfill({ status: 204, headers });
    if (path === "/tracker") return respond({ items: [{ id: "entry", animeId: anime, title: "Synthetic Release Sky", watchStatus: "WATCHING", episodesWatched: 1 }] });
    if (path === "/follows") return respond({ items: follows.get(token) });
    if (path === "/follows/" + anime) {
      follows.set(token, request.method() === "DELETE" ? [] : [{ animeId: anime, title: "Synthetic Release Sky", track: request.postDataJSON().track }]);
      return route.fulfill({ status: 204, headers });
    }
    if (path === "/notifications") {
      const items = notices.get(token);
      if (holdFirst && token.endsWith("1")) {
        blocked = true;
        await new Promise<void>(resolve => { release = resolve; });
      }
      try { return await respond({ items }); } catch { return; }
    }
    if (path === "/notifications/notice-one/read") {
      for (const notice of notices.get(token) ?? []) notice.readAt = new Date().toISOString();
      return route.fulfill({ status: 204, headers });
    }
    return respond({ error: "not_found" }, 404);
  });
  return { calls, release: () => release(), blocked: () => blocked };
}
async function login(page: Page, email: string) {
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill("synthetic-release-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
}
test("release subscriptions, tracks, read state and account reset", async ({ page }) => {
  const api = await mockReleases(page);
  await page.goto("/");
  await login(page, "release-one@example.test");
  const panel = page.getByRole("region", { name: "Release alerts", exact: true });
  await panel.getByRole("button", { name: "Manage release alerts", exact: true }).click();
  await expect(panel.getByText("Private synthetic release for account one")).toBeVisible();
  await panel.getByLabel("Release track Synthetic Release Sky", { exact: true }).selectOption("DUB");
  await panel.getByRole("button", { name: "Follow releases Synthetic Release Sky", exact: true }).click();
  await expect(panel.getByRole("button", { name: "Stop alerts Synthetic Release Sky", exact: true })).toBeVisible();
  expect(api.calls.some(call => call.path === "/follows/" + anime && call.method === "PUT" && (call.body as { track: string }).track === "DUB")).toBe(true);
  await panel.getByRole("button", { name: "Mark as read", exact: true }).click();
  await expect(panel.getByText("Unread", { exact: true })).toHaveCount(0);
  await panel.getByRole("button", { name: "Stop alerts Synthetic Release Sky", exact: true }).click();
  await expect(panel.getByRole("button", { name: "Follow releases Synthetic Release Sky", exact: true })).toBeVisible();
  await page.screenshot({ path: "test-results/release-alerts.png", fullPage: true });
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(panel).toHaveCount(0);
  await login(page, "release-two@example.test");
  await panel.getByRole("button", { name: "Manage release alerts", exact: true }).click();
  await expect(panel.getByText("No release notifications yet.", { exact: true })).toBeVisible();
  await expect(page.getByText("Private synthetic release for account one")).toHaveCount(0);
  expect(await page.evaluate(() => ({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) }))).toEqual({ local: [], session: [] });
});
test("late notification results do not cross accounts", async ({ page }) => {
  const api = await mockReleases(page, true);
  await page.goto("/");
  await login(page, "release-late-one@example.test");
  await page.getByRole("button", { name: "Manage release alerts", exact: true }).click();
  await expect.poll(api.blocked).toBe(true);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page, "release-late-two@example.test");
  api.release();
  await page.getByRole("button", { name: "Manage release alerts", exact: true }).click();
  await expect(page.getByText("No release notifications yet.", { exact: true })).toBeVisible();
  await expect(page.getByText("Private synthetic release for account one")).toHaveCount(0);
});
