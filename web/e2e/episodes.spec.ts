import { test, expect } from "@playwright/test";

test("exact episode editor replaces counts explicitly and preserves gaps", async ({ page }) => {
  const id = "20000000-0000-4000-8000-000000000001";
  let numbers: number[] | null = null;
  let count = 3;
  const submitted: unknown[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
    };
    const respond = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: true });
    if (path === "/anime") return respond({ items: [] });
    if (path === "/auth/login") return respond({ token: "a".repeat(43), user: { id: "one", email: "episodes-browser@example.test", displayName: "", emailVerified: true }, expiresAt: new Date(Date.now() + 86_400_000).toISOString() });
    if (path === "/tracker") return respond({ items: [{ id, animeId: "10000000-0000-4000-8000-000000000001", title: "Synthetic Episode Sky", watchStatus: "WATCHING", episodesWatched: count, exactProgress: numbers !== null, totalEpisodes: 12 }] });
    if (path === "/tracker/" + id + "/episodes") {
      if (request.method() === "PUT") { submitted.push(request.postDataJSON()); numbers = request.postDataJSON().numbers as number[]; count = numbers.length; }
      return respond({ mode: numbers === null ? "COUNT" : "EXACT", count, numbers, total: 12 });
    }
    return route.fulfill({ status: 404, headers, contentType: "application/json", body: "{}" });
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("episodes-browser@example.test");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-episodes-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  const article = page.getByRole("article").filter({ hasText: "Synthetic Episode Sky" });
  await article.getByRole("button", { name: "Edit watched episodes Synthetic Episode Sky", exact: true }).click();
  await expect(article.getByText("Count mode: 3 watched. Episode identities are unknown; saving replaces this count.")).toBeVisible();
  const input = article.getByLabel("Watched episode numbers Synthetic Episode Sky", { exact: true });
  await expect(input).toHaveValue("");
  await input.fill("4, 1, 2");
  await article.getByRole("button", { name: "Replace episode selection", exact: true }).click();
  await expect(article.getByText("Exact episodes tracked; use the episode editor to change your selection.")).toBeVisible();
  expect(submitted).toEqual([{ numbers: [1, 2, 4] }]);
  await expect(article.getByRole("button", { name: "+1 episode", exact: true })).toBeDisabled();
  await article.getByRole("button", { name: "Edit watched episodes Synthetic Episode Sky", exact: true }).click();
  await expect(input).toHaveValue("1, 2, 4");
  await input.fill("1, 1");
  await article.getByRole("button", { name: "Replace episode selection", exact: true }).click();
  await expect(article.getByRole("alert")).toContainText("unique episode numbers");
  expect(submitted).toHaveLength(1);
  await input.fill("");
  await article.getByRole("button", { name: "Replace episode selection", exact: true }).click();
  await expect(article.getByText("0/12 episodes watched")).toBeVisible();
  expect(submitted).toEqual([{ numbers: [1, 2, 4] }, { numbers: [] }]);
});
