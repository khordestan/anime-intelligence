import { expect, test } from "@playwright/test";

test("bulk edits submit selected viewing passes, recover from conflicts, and advance count progress", async ({ page }) => {
  const entries = [
    { id: "entry-one", animeId: "anime-one", title: "Synthetic First", watchStatus: "WATCHING", episodesWatched: 1, watchCycle: 2 },
    { id: "entry-two", animeId: "anime-two", title: "Synthetic Second", watchStatus: "WATCHING", episodesWatched: 2, watchCycle: 1 },
  ];
  const calls: unknown[] = [];
  const ratingCalls: unknown[] = [];
  const progressCalls: unknown[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PATCH, OPTIONS",
    };
    const send = (body: unknown, status = 200) => route.fulfill({ status, headers, json: body });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return send({ authenticationRequired: true });
    if (path === "/auth/login") return send({
      token: "synthetic-test-token",
      expiresAt: new Date(Date.now() + 60000).toISOString(),
      user: { id: "synthetic-owner", email: "bulk@example.test", emailVerified: true },
    });
    if (path === "/tracker") return send({ items: entries });
    if (path === "/tracker/bulk-rating") {
      expect(request.method()).toBe("PATCH");
      ratingCalls.push(request.postDataJSON());
      return send({ items: entries });
    }
    if (path === "/tracker/bulk-progress") {
      expect(request.method()).toBe("PATCH");
      progressCalls.push(request.postDataJSON());
      for (const entry of entries) entry.episodesWatched++;
      return send({ items: entries });
    }
    if (path === "/tracker/bulk-status") {
      expect(request.method()).toBe("PATCH");
      calls.push(request.postDataJSON());
      if (calls.length === 1) {
        entries[0].watchCycle = 3;
        return send({ error: "rewatch_state_changed" }, 409);
      }
      for (const entry of entries) entry.watchStatus = "ON_HOLD";
      return send({ items: entries });
    }
    return send({ items: [] });
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("bulk@example.test");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  const library = page.getByRole("region", { name: "My Library" });
  await library.getByRole("button", { name: "Edit multiple entries" }).click();
  const first = library.getByRole("checkbox", { name: "Select Synthetic First for bulk edit" });
  const second = library.getByRole("checkbox", { name: "Select Synthetic Second for bulk edit" });
  await first.check();
  await second.check();
  await library.getByLabel("Bulk watch status").selectOption("ON_HOLD");
  const apply = library.getByRole("button", { name: "Apply status to selected" });
  await apply.click();
  await expect(library.getByText("The selected entries changed. Select them again and retry.")).toBeVisible();
  await expect(first).not.toBeChecked();
  await first.check();
  await second.check();
  await apply.click();
  await expect(library.getByRole("button", { name: "Edit multiple entries" })).toBeVisible();
  expect(calls).toEqual([2, 3].map(expectedCycle => ({
    watchStatus: "ON_HOLD",
    entries: [{ id: "entry-one", expectedCycle }, { id: "entry-two", expectedCycle: 1 }],
  })));
  await library.getByRole("button", { name: "Edit multiple entries" }).click();
  await second.check();
  await library.getByLabel("Bulk series rating").selectOption("4");
  await library.getByRole("button", { name: "Apply rating to selected" }).click();
  await expect(library.getByRole("button", { name: "Edit multiple entries" })).toBeVisible();
  expect(ratingCalls).toEqual([{ score: 4, entries: [{ id: "entry-two", expectedCycle: 1 }] }]);
  await library.getByRole("button", { name: "Edit multiple entries" }).click();
  await first.check();
  await second.check();
  await library.getByRole("button", { name: "Add one watched episode to selected" }).click();
  await expect(library.getByRole("button", { name: "Edit multiple entries" })).toBeVisible();
  expect(progressCalls).toEqual([{ entries: [
    { id: "entry-one", expectedCycle: 3, expectedEpisodesWatched: 1 },
    { id: "entry-two", expectedCycle: 1, expectedEpisodesWatched: 2 },
  ] }]);
});
