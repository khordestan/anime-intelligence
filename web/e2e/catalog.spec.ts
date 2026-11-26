import { test, expect } from "@playwright/test";

test("catalog search reaches imported aliases and details hide spoilers until requested", async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000099";
  const sequel = "10000000-0000-4000-8000-000000000098";
  let searched = false;
  let characterRequests = 0;
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
    };
    const respond = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: false });
    if (path === "/tracker") return respond({ items: [] });
    if (path === "/anime") {
      searched = url.searchParams.get("q") === "Hidden Alias";
      return respond({ items: searched ? [{ id, title: "Beyond First Page", mediaType: "TV" }] : [] });
    }
    if (path === "/anime/" + id + "/character-credits") {
      characterRequests++;
      return respond({ items: [{ name: "Synthetic Hidden Character", role: "Supporting" }], provider: "synthetic" });
    }
    if (path === "/anime/" + id + "/details") return respond({
      anime: { id, title: "Beyond First Page", nativeTitle: "Synthetic Native Title", synopsis: "Synthetic spoiler synopsis", episodeCountKnown: false },
      seasonNumber: 2, partNumber: 1,
      staffCredits: [{ name: "Synthetic Director", role: "Director" }], hasCharacterCredits: true,
      externalIds: [{ provider: "synthetic", externalId: "work-99" }, { provider: "synthetic-secondary", externalId: "linked-99" }], provider: "synthetic", observedAt: "2026-01-01T00:00:00Z", synonyms: ["Hidden Alias"], genres: ["Adventure"], studios: ["Synthetic Studio"], tags: ["Synthetic spoiler tag"], themes: ["Synthetic spoiler theme"], demographics: ["Synthetic audience"], relations: [{ animeId: sequel, type: "SEQUEL", title: "Synthetic Linked Season", mediaType: "TV", status: "ANNOUNCED" }],
      episodes: [
        { number: 1, track: "SUB", scheduledAt: "2030-01-01T00:00:00Z", revision: 1 },
        { number: 2, track: "RAW", releasedAt: "2026-01-01T00:00:00Z", revision: 2 },
      ],
    });
    if (path === "/anime/" + sequel + "/details") return respond({
      anime: { id: sequel, title: "Synthetic Linked Season", episodeCountKnown: false },
      provider: "synthetic", genres: [], studios: [], tags: [], synonyms: [], episodes: [],
      relations: [{ animeId: id, type: "PREQUEL", title: "Beyond First Page", mediaType: "TV", status: "FINISHED" }],
    });
    return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
  });
  await page.goto("/");
  await page.getByPlaceholder("Search starter catalog…").fill("Hidden Alias");
  await page.getByRole("button", { name: "View catalog details Beyond First Page", exact: true }).click();
  expect(searched).toBe(true);
  const panel = page.getByRole("region", { name: "Catalog details Beyond First Page", exact: true });
  await expect(panel.getByText("Season 2 · Part 1", { exact: true })).toBeVisible();
  await expect(panel.getByText("Genres: Adventure", { exact: true })).toBeVisible();
  await expect(panel.getByText("Demographics: Synthetic audience", { exact: true })).toBeVisible();
  await expect(panel.getByText("Studios: Synthetic Studio", { exact: true })).toBeVisible();
  await expect(panel.getByRole("region", { name: "Staff credits" }).getByText("Synthetic Director · Director")).toBeVisible();
  expect(characterRequests).toBe(0);
  await expect(panel.getByText("Synthetic Hidden Character", { exact: false })).toHaveCount(0);
  await panel.getByRole("button", { name: "Show character credits (may contain spoilers)", exact: true }).click();
  const characters = panel.getByRole("region", { name: "Character credits", exact: true });
  await expect(characters.getByText("Synthetic Hidden Character · Supporting", { exact: true })).toBeVisible();
  await expect(characters.getByText("Credits source: synthetic", { exact: true })).toBeVisible();
  await panel.getByRole("button", { name: "Hide character credits", exact: true }).click();
  await expect(characters).toHaveCount(0);
  expect(characterRequests).toBe(1);
  await expect(panel.getByText(/Episode total unknown/)).toBeVisible();
  await expect(panel.getByText(/Source: synthetic/)).toBeVisible();
  await expect(panel.getByText("Provider IDs: synthetic: work-99 · synthetic-secondary: linked-99", { exact: true })).toBeVisible();
  await expect(panel.getByText(/Episode 1 · SUB · Scheduled/)).toBeVisible();
  await expect(panel.getByText(/Episode 2 · RAW · Confirmed/)).toBeVisible();
  await expect(page.getByText("Synthetic spoiler synopsis", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Tags: Synthetic spoiler tag", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Themes: Synthetic spoiler theme", { exact: true })).toHaveCount(0);
  await panel.getByRole("button", { name: "Show synopsis, themes and tags (may contain spoilers)", exact: true }).click();
  await expect(panel.getByText("Synthetic spoiler synopsis", { exact: true })).toBeVisible();
  await expect(panel.getByText("Tags: Synthetic spoiler tag", { exact: true })).toBeVisible();
  await expect(panel.getByText("Themes: Synthetic spoiler theme", { exact: true })).toBeVisible();
  await panel.getByRole("button", { name: "Hide synopsis, themes and tags", exact: true }).click();
  await expect(page.getByText("Synthetic spoiler synopsis", { exact: true })).toHaveCount(0);
  await panel.getByRole("button", { name: "View sequel: Synthetic Linked Season · TV · ANNOUNCED", exact: true }).click();
  await expect(panel.getByRole("heading", { name: "Synthetic Linked Season", exact: true })).toBeVisible();
  await expect(panel.getByText("Season 2 · Part 1", { exact: true })).toHaveCount(0);
  await panel.getByRole("button", { name: "View prequel: Beyond First Page · TV · FINISHED", exact: true }).click();
  await expect(panel.getByRole("heading", { name: "Beyond First Page", exact: true })).toBeVisible();
  await expect(panel.getByText("Synthetic spoiler synopsis", { exact: true })).toHaveCount(0);
});
