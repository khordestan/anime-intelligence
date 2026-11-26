import { expect, test } from "@playwright/test";

for (const assisted of [false, true]) {
test(`${assisted ? "model-assisted" : "deterministic"} catalog questions show reviewed evidence`, async ({ page }) => {
  const id = "10000000-0000-4000-8000-000000000001";
  const title = "Synthetic evidence";
  const secondID = "10000000-0000-4000-8000-000000000002";
  const secondTitle = "Synthetic comparison";
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
    if (path === "") return reply({ authenticationRequired: assisted, assistedCatalogAnalysisAvailable: assisted });
    if (path === "/auth/login") return reply({
      token: "a".repeat(43),
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
      user: { id: "analyst", email: "analyst@example.test", displayName: "Analyst", emailVerified: true },
    });
    if (path === "/anime") return reply({
      items: url.searchParams.get("q") === "Synthetic" ? [{ id, title }, { id: secondID, title: secondTitle }] : [],
    });
    if (path === "/tracker") return reply({ items: [{ id: "entry", animeId: id, title, watchStatus: "PLANNING", episodesWatched: 0 }] });
    if (path === "/anime/" + id + "/details") {
      return reply({
        anime: { id, title, episodeCountKnown: true, synopsis: "Synthetic spoiler" },
        genres: [], studios: [], tags: [], synonyms: [], relations: [], episodes: [],
      });
    }
    if (path === (assisted ? "/analysis/catalog/assisted" : "/analysis/catalog")) {
      if (assisted) expect(request.headers().authorization).toBe("Bearer " + "a".repeat(43));
      const payload = request.postDataJSON();
      if (!assisted && payload.animeIds.length === 2) {
        expect(payload).toEqual({ question: "How many episodes and which genres?", animeIds: [id, secondID] });
        return reply({ engine: "catalog-facts-v1", topicMatched: true, comparisons: [{field: "episodeCount", difference: -12, animeIds: [id, secondID]}], sharedEvidence: [{ field: "genres", values: ["Adventure"] }], items: [
          { animeId: id, title, evidence: [{ field: "episodeCount", values: ["0"] }, { field: "genres", values: ["Adventure", "Drama"] }] },
          { animeId: secondID, title: secondTitle, evidence: [{ field: "episodeCount", values: ["12"] }, { field: "genres", values: ["Adventure"] }] },
        ] });
      }
      if (payload.question === "What has been confirmed and what is scheduled?") {
        expect(payload.animeIds).toEqual([id]);
        return reply({ engine: "catalog-facts-v1", topicMatched: true, items: [{ animeId: id, title, evidence: [
          { field: "confirmedReleases", values: [], episodeFacts: [{ number: 1, track: "SUB", time: "2026-01-01T00:00:00Z", provider: "synthetic-confirmed" }] },
          { field: "upcomingSchedule", values: [], episodeFacts: [{ number: 2, track: "DUB", time: "2027-01-01T00:00:00Z", provider: "synthetic-prediction" }] },
        ] }] });
      }
      if (payload.question === "Is the selected track scheduled?") {
        expect(payload).toEqual({ question: "Is the selected track scheduled?", animeIds: [id], releaseScope: { episodeNumber: 2, track: "DUB" } });
        return reply({ engine: "catalog-facts-v1", topicMatched: true, items: [{ animeId: id, title, evidence: [
          { field: "upcomingSchedule", values: [], episodeFacts: [{ number: 2, track: "DUB", time: "2027-01-01T00:00:00Z", provider: "synthetic-scoped" }] },
        ] }] });
      }
      if (payload.question === "Which sequel is recorded?") {
        expect(payload.animeIds).toEqual([id]);
        return reply({ engine: "catalog-facts-v1", topicMatched: true, items: [{ animeId: id, title, evidence: [
          { field: "continuity", values: [], truncated: true, continuityFacts: [
            { animeId: secondID, title: secondTitle, relation: "SEQUEL", providers: ["synthetic-continuity"] },
          ] },
        ] }] });
      }
      expect(payload).toEqual({ question: "How many episodes?", animeIds: [id] });
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
  if (!assisted) {
    await page.getByLabel("Question about this title").fill("What has been confirmed and what is scheduled?");
    await page.getByRole("button", { name: "Show evidence" }).click();
    await expect(evidence.getByText("Confirmed releases", { exact: true })).toBeVisible();
    await expect(evidence.getByText("Upcoming schedule (unconfirmed)", { exact: true })).toBeVisible();
    await expect(evidence.getByText("Source: synthetic-confirmed")).toBeVisible();
    await expect(evidence.getByText("Source: synthetic-prediction")).toBeVisible();
    await page.getByLabel("Question about this title").fill("Which sequel is recorded?");
    await page.getByRole("button", { name: "Show evidence" }).click();
    await expect(evidence.getByText("Direct continuity", { exact: true })).toBeVisible();
    await expect(evidence.getByText("Sequel · " + secondTitle)).toBeVisible();
    await expect(evidence.getByText("Source: synthetic-continuity")).toBeVisible();
    await expect(evidence.getByText("Continuity evidence is limited")).toBeVisible();
    await page.getByText("Narrow to release evidence", { exact: true }).click();
    const episode = page.getByLabel("Episode number", { exact: true });
    const track = page.getByRole("combobox", { name: "Release track", exact: true });
    await episode.fill("2");
    await track.selectOption("DUB");
    await page.getByLabel("Question about this title").fill("Is the selected track scheduled?");
    await page.getByRole("button", { name: "Show evidence" }).click();
    await expect(evidence.getByText("Source: synthetic-scoped")).toBeVisible();
    await episode.clear();
    await track.selectOption("");
    await page.getByLabel("Question about this title").fill("How many episodes?");
  }
  if (!assisted) {
    await page.getByText("Compare with another title", { exact: true }).click();
    await page.getByLabel("Find a comparison title").fill("Synthetic");
    await page.getByRole("button", { name: "Search titles", exact: true }).click();
    await page.getByRole("button", { name: secondTitle, exact: true }).click();
    await page.getByLabel("Question about these titles").fill("How many episodes and which genres?");
    await page.getByRole("button", { name: "Show evidence" }).click();
    await expect(evidence.getByRole("heading", { name: title, exact: true })).toBeVisible();
    await expect(evidence.getByRole("heading", { name: secondTitle, exact: true })).toBeVisible();
    await expect(evidence.getByText("12", { exact: true })).toBeVisible();
    await expect(evidence.getByText(title + " has 12 fewer episodes than " + secondTitle + ".")).toBeVisible();
    await expect(evidence.getByText("Shared genres", { exact: true }).locator("..")).toContainText("Adventure");
    await page.getByRole("button", { name: "Remove comparison" }).click();
    await expect(evidence).toHaveCount(0);
  }
});
}
