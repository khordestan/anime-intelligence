import { test, expect } from "@playwright/test";

test("catalog search reaches imported aliases and details hide spoilers until requested", async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000099";
  let searched = false;
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
    if (path === "/anime/" + id + "/details") return respond({
      anime: { id, title: "Beyond First Page", nativeTitle: "Synthetic Native Title", synopsis: "Synthetic spoiler synopsis", episodeCountKnown: false },
      provider: "synthetic", observedAt: "2026-01-01T00:00:00Z", synonyms: ["Hidden Alias"], genres: ["Adventure"], studios: ["Synthetic Studio"], tags: ["Synthetic spoiler tag"], relations: [],
      episodes: [
        { number: 1, track: "SUB", scheduledAt: "2030-01-01T00:00:00Z", revision: 1 },
        { number: 2, track: "RAW", releasedAt: "2026-01-01T00:00:00Z", revision: 2 },
      ],
    });
    return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
  });
  await page.goto("/");
  await page.getByPlaceholder("Search starter catalog…").fill("Hidden Alias");
  await page.getByRole("button", { name: "View catalog details Beyond First Page", exact: true }).click();
  expect(searched).toBe(true);
  const panel = page.getByRole("region", { name: "Catalog details Beyond First Page", exact: true });
  await expect(panel.getByText("Genres: Adventure", { exact: true })).toBeVisible();
  await expect(panel.getByText("Studios: Synthetic Studio", { exact: true })).toBeVisible();
  await expect(panel.getByText(/Episode total unknown/)).toBeVisible();
  await expect(panel.getByText(/Source: synthetic/)).toBeVisible();
  await expect(panel.getByText(/Episode 1 · SUB · Scheduled/)).toBeVisible();
  await expect(panel.getByText(/Episode 2 · RAW · Confirmed/)).toBeVisible();
  await expect(page.getByText("Synthetic spoiler synopsis", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Tags: Synthetic spoiler tag", { exact: true })).toHaveCount(0);
  await panel.getByRole("button", { name: "Show synopsis and tags (may contain spoilers)", exact: true }).click();
  await expect(panel.getByText("Synthetic spoiler synopsis", { exact: true })).toBeVisible();
  await expect(panel.getByText("Tags: Synthetic spoiler tag", { exact: true })).toBeVisible();
  await panel.getByRole("button", { name: "Hide synopsis and tags", exact: true }).click();
  await expect(page.getByText("Synthetic spoiler synopsis", { exact: true })).toHaveCount(0);
});
