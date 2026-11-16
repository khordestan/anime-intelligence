import { test, expect, type Page } from "@playwright/test";

async function mockCalendar(page: Page, hold = false) {
  let issued = 0, blocked = false;
  let release = () => {};
  const windows: { from: string | null; days: string | null }[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace("Bearer ", "");
    const headers = { "Access-Control-Allow-Origin": "http://127.0.0.1:3000", "Access-Control-Allow-Headers": "Authorization, Content-Type", "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS" };
    const respond = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: true });
    if (path === "/auth/login") { const id = String(++issued); return respond({ token: id.padStart(43,"a"), user: { id, email: request.postDataJSON().email, emailVerified: true }, expiresAt: new Date(Date.now()+86_400_000).toISOString() }); }
    if (path === "/auth/logout") return route.fulfill({ status: 204, headers });
    if (path === "/anime" || path === "/tracker") return respond({ items: [] });
    if (path === "/calendar") {
      const from = url.searchParams.get("from")!, days = url.searchParams.get("days")!;
      windows.push({ from, days });
      const start = new Date(from + "T00:00:00Z"), end = new Date(start.getTime() + Number(days)*86_400_000);
      const items = token.endsWith("1") && days !== "1" ? [{ animeId: "synthetic", title: "Synthetic Followed Schedule", number: 2, track: "SUB", scheduledAt: new Date(start.getTime()+3_600_000).toISOString(), revision: 3, provider: "synthetic-provider" }] : [];
      if (hold && token.endsWith("1")) { blocked = true; await new Promise<void>(resolve => { release = resolve; }); }
      try { return await respond({ from: start.toISOString(), to: end.toISOString(), items, limit: 100, truncated: false }); } catch { return; }
    }
    return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
  });
  return { windows, blocked: () => blocked, release: () => release() };
}
async function login(page: Page, email: string) {
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill("synthetic-calendar-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
}
test("follow calendar labels predictions and applies bounded date controls", async ({ page }) => {
  const api = await mockCalendar(page);
  await page.goto("/"); await login(page,"calendar-browser@example.test");
  const panel = page.getByRole("region", { name: "Followed release calendar", exact: true });
  await panel.getByRole("button", { name: "Upcoming followed schedules", exact: true }).click();
  await expect(panel.getByText("Synthetic Followed Schedule · Episode 2 · SUB", { exact: true })).toBeVisible();
  await expect(panel.getByText(/Unconfirmed · Revision 3/)).toBeVisible();
  await expect(panel.getByText("Source: synthetic-provider", { exact: true })).toBeVisible();
  await panel.getByLabel("Calendar start date (UTC)", { exact: true }).fill("2026-12-01");
  await panel.getByLabel("Calendar days", { exact: true }).selectOption("1");
  await panel.getByRole("button", { name: "Refresh followed schedules", exact: true }).click();
  await expect(panel.getByText(/No unconfirmed schedules in this window/)).toBeVisible();
  expect(api.windows[api.windows.length - 1]).toEqual({ from: "2026-12-01", days: "1" });
  await expect(panel.getByText("Synthetic Followed Schedule · Episode 2 · SUB", { exact: true })).toHaveCount(0);
  expect(await page.evaluate(() => ({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) }))).toEqual({ local: [], session: [] });
});
test("late followed schedules cannot cross accounts", async ({ page }) => {
  const api = await mockCalendar(page,true);
  await page.goto("/"); await login(page,"calendar-first@example.test");
  await page.getByRole("button", { name: "Upcoming followed schedules", exact: true }).click();
  await expect.poll(api.blocked).toBe(true);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page,"calendar-second@example.test");
  api.release();
  await page.getByRole("button", { name: "Upcoming followed schedules", exact: true }).click();
  await expect(page.getByText(/No unconfirmed schedules in this window/)).toBeVisible();
  await expect(page.getByText("Synthetic Followed Schedule · Episode 2 · SUB", { exact: true })).toHaveCount(0);
});
