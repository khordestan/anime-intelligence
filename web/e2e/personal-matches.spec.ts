import { test, expect, type Page } from "@playwright/test";

async function mockMatches(page: Page, hold = false) {
  let issued = 0;
  const preferences = new Map<string, number | null>();
  const calls: { path: string; token: string; body: unknown }[] = [];
  let release = () => {};
  let blocked = false;
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace("Bearer ", "");
    const headers = { "Access-Control-Allow-Origin": "http://127.0.0.1:3000", "Access-Control-Allow-Headers": "Authorization, Content-Type", "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS" };
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    calls.push({ path, token, body: request.postData() ? request.postDataJSON() : undefined });
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
    if (path === "/recommendations") {
      const items = preferences.get(token) === null ? [] : [{ animeId: "synthetic-match", title: "Synthetic account match", score: 90, evidence: [{ featureKey: "synthetic.mystery", direction: "match", strength: 1, impact: 1 }] }];
      if (hold && token.endsWith("1")) { blocked = true; await new Promise<void>(resolve => { release = resolve; }); }
      try { return await respond({ items, profile: items.length ? [{ key: "synthetic.mystery", declared: { weight: preferences.get(token), confidence: 1 }, inferred: { weight: 0.5, confidence: 0.25 } }] : [] }); } catch { return; }
    }
    return respond({}, 404);
  });
  return { calls, blocked: () => blocked, release: () => release() };
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
