import { expect, test } from "@playwright/test";

test("Global Essentials loads publicly and links to the evidence", async ({ page }) => {
  await page.route("**/api/v1", (route) => route.fulfill({
    json: { authenticationRequired: true, globalEssentialsAvailable: true },
  }));
  await page.route("**/api/v1/anime**", (route) => route.fulfill({ json: { items: [] } }));
  await page.route("**/api/v1/recommendations/essentials", (route) => {
    expect(route.request().headers().authorization).toBeUndefined();
    return route.fulfill({
      json: {
        algorithmVersion: "global-essentials-v1",
        items: [{
          animeId: "example",
          title: "Example Anime",
          evidence: [{ kind: "JURIED_AWARD_WIN", publisherId: "Test Academy", sourceUrl: "https://award.example/results" }],
        }],
      },
    });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Browse Global Essentials" }).click();
  await expect(page.getByRole("heading", { name: "Example Anime" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Juried award win · Test Academy" })).toHaveAttribute("href", "https://award.example/results");
});
