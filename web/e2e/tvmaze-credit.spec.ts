import { test, expect } from "@playwright/test";

for (const metadataProvider of ["tvmaze", "wikidata"]) {

test("TVmaze schedules keep attribution with " + metadataProvider + " metadata and never confirm air dates", async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000077";
  const title = "Synthetic anime — Season 2";
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
      provider: metadataProvider, observedAt: "2026-01-02T00:00:00Z",
      externalIds: [{ provider: "tvmaze", externalId: "show-101-season-501" }],
      genres: [], studios: [], tags: [], synonyms: [], relations: [],
      episodes: [{ provider: "tvmaze", number: 1, track: "UNKNOWN", scheduledAt: "2020-01-01T00:00:00Z", revision: 1 }],
    });
    return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "View catalog details " + title, exact: true }).click();
  const panel = page.getByRole("region", { name: "Catalog details " + title, exact: true });
  await expect(panel.getByRole("link", { name: "TVmaze", exact: true })).toHaveAttribute("href", "https://www.tvmaze.com/");
  await expect(panel.getByRole("link", { name: "CC BY-SA 4.0", exact: true })).toHaveAttribute("href", "https://creativecommons.org/licenses/by-sa/4.0/");
  await expect(panel.getByText(/Data from TVmaze, adapted under/)).toBeVisible();
  await expect(panel.getByText(/Episode total unknown/)).toBeVisible();
  const episode = panel.getByText(/Episode 1 · UNKNOWN · Scheduled/);
  await expect(episode).toBeVisible();
  await expect(panel.getByText(/Revision 1 · Source: tvmaze/)).toBeVisible();
  await expect(episode).not.toContainText("Confirmed");
  await expect(page.locator("footer").getByRole("link", { name: "TVmaze", exact: true })).toBeVisible();
});

}
