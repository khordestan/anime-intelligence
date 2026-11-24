import { expect, test } from "@playwright/test";

test("catalog questions show the selected fact and its reviewed source", async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000001";
  const title = "Synthetic evidence";
  await page.route("**/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    };
    const reply = (json: unknown, status = 200) => route.fulfill({ status, headers, json });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return reply({ authenticationRequired: false });
    if (path === "/anime") return reply({ items: [] });
    if (path === "/tracker") return reply({ items: [{ id: "entry", animeId: id, title, watchStatus: "PLANNING", episodesWatched: 0 }] });
    if (path === "/anime/" + id + "/details") {
      return reply({
        anime: { id, title, episodeCountKnown: true, synopsis: "Synthetic spoiler" },
        genres: [], studios: [], tags: [], synonyms: [], relations: [], episodes: [],
      });
    }
    if (path === "/analysis/catalog") {
      expect(request.postDataJSON()).toEqual({ question: "How many episodes?", animeIds: [id] });
      return reply({
        engine: "catalog-facts-v1", topicMatched: true, items: [
          { animeId: id, title, evidence: [{ field: "episodeCount", values: ["0"], provider: "synthetic-review" }] },
        ],
      });
    }
    return reply({ error: "not_found" }, 404);
  });
  await page.goto("/");
  await page.getByRole("button", { name: "View catalog details " + title }).click();
  await page.getByText("Ask about catalog facts", { exact: true }).click();
  await page.getByLabel("Question about this title").fill("How many episodes?");
  await page.getByRole("button", { name: "Show evidence" }).click();
  const evidence = page.getByRole("region", { name: "Catalog evidence" });
  await expect(evidence.getByText("Episode count", { exact: true })).toBeVisible();
  await expect(evidence.getByText("0", { exact: true })).toBeVisible();
  await expect(evidence.getByText("Source: synthetic-review")).toBeVisible();
  await expect(page.getByText("Synthetic spoiler")).toHaveCount(0);
});
