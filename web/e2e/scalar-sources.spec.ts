import { test, expect } from "@playwright/test";

for (const total of [null, 0, 12]) {
  test("reviewed scalar metadata keeps values and provenance consistent for total " + total, async ({ page }) => {
    const id = "10000000-0000-4000-8000-000000000079";
    const title = "Synthetic scalar-reviewed title";
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
        anime: { id, title, mediaType: "MOVIE", season: "SPRING", releaseYear: 2028, episodeCountKnown: total !== null, episodeCount: total ?? undefined },
        provider: "synthetic-primary", observedAt: "2026-01-02T00:00:00Z",
        genres: [], studios: [], tags: [], synonyms: [], relations: [], episodes: [],
        fieldSources: ["releaseYear", "season", "mediaType", "episodeCount"].map(field => ({
          field, mode: "SELECT", revision: 1, provider: "synthetic-secondary", observedAt: "2026-01-01T00:00:00Z",
        })),
      });
      return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
    });
    await page.goto("/");
    await page.getByRole("button", { name: "View catalog details " + title, exact: true }).click();
    const panel = page.getByRole("region", { name: "Catalog details " + title, exact: true });
    await expect(panel.getByText(/MOVIE · SPRING 2028/)).toBeVisible();
    if (total === null) await expect(panel.getByText(/Episode total unknown/)).toBeVisible();
    else await expect(panel.getByText(new RegExp(total + " episodes$"))).toBeVisible();
    const sources = panel.getByRole("list", { name: "Field sources" });
    for (const field of ["Release year", "Airing quarter", "Format", "Episode total"])
      await expect(sources.getByText(new RegExp(field + ": synthetic-secondary"))).toBeVisible();
    await expect(panel.getByText(/Source: synthetic-primary/)).toBeVisible();
  });
}
