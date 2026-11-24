import { expect, test } from "@playwright/test";

for (const assisted of [false, true]) {
test(`${assisted ? "model-assisted" : "deterministic"} catalog questions show reviewed evidence`, async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000001";
  const title = "Synthetic evidence";
  await page.route("**/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Content-Type, Authorization",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    };
    const reply = (json: unknown, status = 200) => route.fulfill({ status, headers, json });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return reply({ authenticationRequired: assisted, assistedCatalogAnalysisAvailable: assisted });
    if (path === "/auth/login") return reply({
      token: "a".repeat(43),
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
      user: { id: "analyst", email: "analyst@example.test", displayName: "Analyst", emailVerified: true },
    });
    if (path === "/anime") return reply({ items: [] });
    if (path === "/tracker") return reply({ items: [{ id: "entry", animeId: id, title, watchStatus: "PLANNING", episodesWatched: 0 }] });
    if (path === "/anime/" + id + "/details") {
      return reply({
        anime: { id, title, episodeCountKnown: true, synopsis: "Synthetic spoiler" },
        genres: [], studios: [], tags: [], synonyms: [], relations: [], episodes: [],
      });
    }
    if (path === (assisted ? "/analysis/catalog/assisted" : "/analysis/catalog")) {
      if (assisted) expect(request.headers().authorization).toBe("Bearer " + "a".repeat(43));
      expect(request.postDataJSON()).toEqual({ question: "How many episodes?", animeIds: [id] });
      return reply({
        engine: "catalog-facts-v1", model: assisted ? "synthetic-model" : undefined, topicMatched: true, items: [
          { animeId: id, title, evidence: [{ field: "episodeCount", values: ["0"], provider: "synthetic-review" }] },
        ],
      });
    }
    return reply({ error: "not_found" }, 404);
  });
  await page.goto("/");
  if (assisted) {
    await page.getByLabel("Email", { exact: true }).fill("analyst@example.test");
    await page.getByLabel("Password", { exact: true }).fill("synthetic-analyst-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
  }
  await page.getByRole("button", { name: "View catalog details " + title }).click();
  await page.getByText("Ask about catalog facts", { exact: true }).click();
  await page.getByLabel("Question about this title").fill("How many episodes?");
  const mode = page.getByRole("checkbox", { name: "Let OpenAI select topics using your question" });
  if (assisted) {
    await expect(mode).not.toBeChecked();
    await mode.check();
  } else {
    await expect(mode).toHaveCount(0);
  }
  await page.getByRole("button", { name: "Show evidence" }).click();
  const evidence = page.getByRole("region", { name: "Catalog evidence" });
  await expect(evidence.getByText("Episode count", { exact: true })).toBeVisible();
  await expect(evidence.getByText("0", { exact: true })).toBeVisible();
  await expect(evidence.getByText("Source: synthetic-review")).toBeVisible();
  if (assisted) await expect(evidence.getByText("Topics selected by synthetic-model")).toBeVisible();
  await expect(page.getByText("Synthetic spoiler")).toHaveCount(0);
});
}
