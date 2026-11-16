import { test, expect, type Page } from "@playwright/test";

async function mockNotes(page: Page, hold = false) {
  let issued = 0, puts = 0, blocked = false;
  let release = () => {};
  let notes = "Existing synthetic note", updatedAt = "2026-01-01T00:00:00Z";
  const writes: { notes: string; expectedUpdatedAt: string }[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request(), path = new URL(request.url()).pathname.replace("/api/v1", "");
    const token = (request.headers().authorization ?? "").replace("Bearer ", "");
    const headers = { "Access-Control-Allow-Origin": "http://127.0.0.1:3000", "Access-Control-Allow-Headers": "Authorization, Content-Type", "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS" };
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: true });
    if (path === "/anime") return respond({ items: [] });
    if (path === "/auth/login") { const id = String(++issued); return respond({ token: id.padStart(43,"a"), user: { id, email: request.postDataJSON().email, emailVerified: true }, expiresAt: new Date(Date.now() + 86_400_000).toISOString() }); }
    if (path === "/auth/logout") return route.fulfill({ status: 204, headers });
    if (path === "/tracker") return respond({ items: token.endsWith("1") ? [{ id: "synthetic-note-entry", animeId: "synthetic-note-title", title: "Synthetic Notes", watchStatus: "WATCHING", episodesWatched: 1, watchCycle: 1 }] : [] });
    if (path === "/tracker/synthetic-note-entry/notes") {
      if (request.method() === "PUT") {
        writes.push(request.postDataJSON());
        if (++puts === 1) { notes = "Other tab synthetic note"; updatedAt = "2026-01-02T00:00:00Z"; return respond({ error: "tracker_notes_conflict" },409); }
        notes = request.postDataJSON().notes; updatedAt = "2026-01-03T00:00:00Z";
        return respond({ notes, updatedAt });
      }
      if (hold) { blocked = true; await new Promise<void>(resolve => { release = resolve; }); }
      try { return await respond({ notes, updatedAt }); } catch { return; }
    }
    return respond({},404);
  });
  return { writes, blocked: () => blocked, release: () => release() };
}
async function login(page: Page, email: string) {
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Password", { exact: true }).fill("synthetic-notes-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByText(email, { exact: true })).toBeVisible();
}
test("private note conflicts preserve drafts, reload and clear explicitly", async ({ page }) => {
  const api = await mockNotes(page);
  await page.goto("/"); await login(page,"notes-browser@example.test");
  await page.getByRole("button", { name: "Edit private notes Synthetic Notes", exact: true }).click();
  const group = page.getByRole("group", { name: "Private notes Synthetic Notes", exact: true });
  const input = group.getByLabel("Notes Synthetic Notes", { exact: true });
  await expect(input).toHaveValue("Existing synthetic note");
  await input.fill("My synthetic draft");
  await group.getByRole("button", { name: "Save notes", exact: true }).click();
  await expect(group.getByText(/Your draft is kept/)).toBeVisible();
  await expect(input).toHaveValue("My synthetic draft");
  await group.getByRole("button", { name: "Discard draft and reload notes", exact: true }).click();
  await expect(input).toHaveValue("Other tab synthetic note");
  await input.fill("Final synthetic note");
  await group.getByRole("button", { name: "Save notes", exact: true }).click();
  await expect(group).toHaveCount(0);
  expect(api.writes[1]).toEqual({ notes: "Final synthetic note", expectedUpdatedAt: "2026-01-02T00:00:00Z" });
  await page.getByRole("button", { name: "Edit private notes Synthetic Notes", exact: true }).click();
  await expect(input).toHaveValue("Final synthetic note");
  await input.fill("");
  await group.getByRole("button", { name: "Save notes", exact: true }).click();
  await expect(group).toHaveCount(0);
  expect(api.writes[2].notes).toBe("");
  expect(await page.evaluate(() => ({ local: Object.keys(localStorage), session: Object.keys(sessionStorage) }))).toEqual({ local: [], session: [] });
});
test("late private notes cannot cross accounts", async ({ page }) => {
  const api = await mockNotes(page,true);
  await page.goto("/"); await login(page,"notes-first@example.test");
  await page.getByRole("button", { name: "Edit private notes Synthetic Notes", exact: true }).click();
  await expect.poll(api.blocked).toBe(true);
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await login(page,"notes-second@example.test");
  api.release();
  await expect(page.getByRole("group", { name: "Private notes Synthetic Notes", exact: true })).toHaveCount(0);
  await expect(page.getByRole("textbox", { name: "Notes Synthetic Notes", exact: true })).toHaveCount(0);
});
