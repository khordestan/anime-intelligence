import { test, expect } from "@playwright/test";

test("reviewed field sources remain distinct from primary metadata and spoiler values stay hidden", async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000078";
  const title = "Synthetic field-reviewed title";
  const headers = {
    "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
    "Access-Control-Allow-Headers": "Authorization, Content-Type",
    "Access-Control-Allow-Methods": "GET, OPTIONS",
  };
  await page.route("http://localhost:8080/api/v1**", async route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    const send = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return send({ authenticationRequired: false });
    if (path === "/anime") return send({ items: [] });
    if (path === "/tracker") return send({ items: [{ id: "entry", animeId: id, title, watchStatus: "WATCHING", episodesWatched: 0 }] });
    if (path === "/anime/" + id + "/details") return send({
      anime: { id, title, episodeCountKnown: false },
      provider: "synthetic-primary", observedAt: "2026-01-02T00:00:00Z",
      genres: ["Reviewed genre"], themes: ["Synthetic hidden reviewed theme"], studios: [], tags: [], synonyms: [], relations: [], episodes: [],
      fieldSources: [
        { field: "genres", mode: "SELECT", revision: 1, provider: "synthetic-secondary", observedAt: "2026-01-01T00:00:00Z" },
        { field: "themes", mode: "SELECT", revision: 2, provider: "synthetic-secondary", observedAt: "2026-01-01T00:00:00Z" },
      ],
    });
    return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "View catalog details " + title, exact: true }).click();
  const panel = page.getByRole("region", { name: "Catalog details " + title, exact: true });
  await expect(panel.getByText("Genres: Reviewed genre", { exact: true })).toBeVisible();
  await expect(panel.getByText(/Source: synthetic-primary/)).toBeVisible();
  const sources = panel.getByRole("list", { name: "Field sources" });
  await expect(sources.getByText(/genres: synthetic-secondary/)).toBeVisible();
  await expect(sources.getByText(/themes: synthetic-secondary/)).toBeVisible();
  await expect(panel.getByText("Themes: Synthetic hidden reviewed theme", { exact: true })).toHaveCount(0);
  await panel.getByRole("button", { name: "Show synopsis, themes and tags (may contain spoilers)", exact: true }).click();
  await expect(panel.getByText("Themes: Synthetic hidden reviewed theme", { exact: true })).toBeVisible();
});
