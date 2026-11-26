import { expect, test } from "@playwright/test";

test("season rating remains separate from series rating and can be cleared", async ({ page }) => {
  const changes: unknown[] = [];
  let seasonRating: number | undefined;
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PUT, OPTIONS",
    };
    const send = (json: unknown) => route.fulfill({ status: 200, headers, json });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return send({ authenticationRequired: true });
    if (path === "/auth/login") return send({
      token: "synthetic-test-token",
      expiresAt: new Date(Date.now() + 60000).toISOString(),
      user: { id: "synthetic-owner", email: "season@example.test", emailVerified: true },
    });
    const numbered = {
      id: "entry-one", animeId: "anime-one", title: "Synthetic Season Part", watchStatus: "WATCHING",
      episodesWatched: 1, watchCycle: 2, rating: 3, seasonRating, seasonRatingAvailable: true,
    };
    if (path === "/tracker") return send({ items: [numbered, {
      id: "entry-two", animeId: "anime-two", title: "Synthetic Unnumbered Title", watchStatus: "WATCHING",
      episodesWatched: 0, watchCycle: 1,
    }] });
    if (path === "/tracker/entry-one/season-rating") {
      expect(request.method()).toBe("PUT");
      const change = request.postDataJSON();
      changes.push(change);
      seasonRating = change.score ?? undefined;
      return send({ ...numbered, seasonRating });
    }
    return send({ items: [] });
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("season@example.test");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  const series = page.getByLabel("Rating Synthetic Season Part", { exact: true });
  const season = page.getByLabel("Season rating Synthetic Season Part", { exact: true });
  await expect(series).toHaveValue("3");
  await expect(page.getByLabel("Season rating Synthetic Unnumbered Title", { exact: true })).toHaveCount(0);
  await season.selectOption("5");
  await expect(season).toHaveValue("5");
  await expect(season).toBeEnabled();
  await season.selectOption("");
  await expect(season).toHaveValue("");
  await expect(series).toHaveValue("3");
  expect(changes).toEqual([{ score: 5, expectedCycle: 2 }, { score: null, expectedCycle: 2 }]);
});
