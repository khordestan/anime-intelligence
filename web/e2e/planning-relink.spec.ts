import { expect, test } from "@playwright/test";

test("correcting a Planning title reviews the exact destination and refreshed entry version", async ({ page }) => {
  const source = { id: "10000000-0000-4000-8000-000000000011", title: "Synthetic duplicate" };
  const target = { id: "10000000-0000-4000-8000-000000000012", title: "Intended synthetic title", releaseYear: 2025, mediaType: "TV" };
  const oldVersion = "2026-01-01T00:00:00Z", reviewedVersion = "2026-01-01T00:01:00Z";
  const requests: unknown[] = [];
  let entry = { id: "old-entry", animeId: source.id, title: source.title, watchStatus: "PLANNING", episodesWatched: 0, updatedAt: oldVersion };
  await page.route("**/api/v1**", async route => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Content-Type, Authorization",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    };
    const reply = (json: unknown, status = 200) => route.fulfill({ status, headers, json });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return reply({ authenticationRequired: true });
    if (path === "/auth/login") return reply({
      token: "a".repeat(43),
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
      user: { id: "synthetic", email: "planner@example.test", emailVerified: true },
    });
    if (path === "/tracker") return reply({ items: [entry] });
    if (path === "/anime") return reply({ items: url.searchParams.get("q") === "synthetic" ? [source, target] : [] });
    if (path === "/tracker/old-entry/relink-planning") {
      expect(request.headers().authorization).toBe("Bearer " + "a".repeat(43));
      requests.push(request.postDataJSON());
      if (requests.length === 1) {
        entry = { ...entry, updatedAt: reviewedVersion };
        return reply({ error: "planning_entry_changed" }, 409);
      }
      entry = { ...entry, id: "new-entry", animeId: target.id, title: target.title };
      return reply(entry);
    }
    return reply({ error: "not_found" }, 404);
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("planner@example.test");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-planning-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  const library = page.getByRole("region", { name: "My Library", exact: true });
  const correction = "Correct planning title for " + source.title;
  async function review() {
    await library.getByRole("button", { name: correction, exact: true }).click();
    const panel = library.getByRole("region", { name: correction, exact: true });
    await panel.getByLabel("Find the intended title").fill("synthetic");
    await panel.getByRole("button", { name: "Search titles", exact: true }).click();
    await panel.getByRole("button", { name: target.title + " · 2025 · TV", exact: true }).click();
    await expect(panel.getByText(/Replace the empty Planning entry/)).toContainText(target.title);
    return panel;
  }
  const initial = await review();
  expect(requests).toEqual([]);
  await initial.getByRole("button", { name: "Confirm planning title correction" }).click();
  await expect(initial.getByRole("alert")).toContainText("Refresh the library");
  await initial.getByRole("button", { name: "Cancel correction" }).click();
  await library.getByRole("button", { name: "Refresh", exact: true }).click();
  const next = await review();
  await next.getByRole("button", { name: "Confirm planning title correction" }).click();
  await expect(library.getByRole("heading", { name: target.title, exact: true })).toBeVisible();
  await expect(library.getByRole("heading", { name: source.title, exact: true })).toHaveCount(0);
  expect(requests).toEqual([
    { targetAnimeId: target.id, expectedUpdatedAt: oldVersion },
    { targetAnimeId: target.id, expectedUpdatedAt: reviewedVersion },
  ]);
});
