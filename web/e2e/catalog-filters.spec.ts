import { test, expect } from "@playwright/test";

const headers = {
  "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
  "Access-Control-Allow-Headers": "Authorization, Content-Type",
  "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
};
const result = { id: "10000000-0000-4000-8000-000000000081", title: "Filtered Imported Anime", mediaType: "TV" };

test("combined catalog filters apply explicitly without a text query and clear", async ({ page }) => {
  const filtered: URL[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    const respond = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: false });
    if (path === "/tracker") return respond({ items: [] });
    if (path === "/anime") {
      if (url.searchParams.has("genre")) { filtered.push(url); return respond({ items: [result, ...Array.from({ length: 9 }, (_, i) => ({ ...result, id: result.id.slice(0, -2) + String(82 + i), title: "Additional Filter Match " + (i + 1) }))] }); }
      return respond({ items: [{ ...result, title: "Unfiltered Starter" }] });
    }
    return route.fulfill({ status: 404, headers, body: "{}" });
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Catalog filters", exact: true }).click();
  const form = page.getByRole("form", { name: "Catalog metadata filters", exact: true });
  await form.getByLabel("Catalog format", { exact: true }).selectOption("TV");
  await form.getByLabel("Catalog sort order", { exact: true }).selectOption("NEWEST");
  await form.getByLabel("Minimum episodes", { exact: true }).fill("12");
  await form.getByLabel("Maximum episodes", { exact: true }).fill("24");
  await form.getByLabel("Catalog release status", { exact: true }).selectOption("RELEASING");
  await form.getByLabel("Catalog release quarter", { exact: true }).selectOption("FALL");
  await form.getByLabel("Catalog release year", { exact: true }).fill("2026");
  await form.getByLabel("Earliest release year", { exact: true }).fill("2020");
  await form.getByLabel("Latest release year", { exact: true }).fill("2030");
  await form.getByLabel("Source genre", { exact: true }).fill("  Action & Power  ");
  await form.getByLabel("Source theme (may contain spoilers)", { exact: true }).fill("Strategy");
  await form.getByLabel("Source demographic", { exact: true }).fill("Seinen");
  await form.getByLabel("Source studio", { exact: true }).fill("Synthetic Works");
  expect(filtered).toHaveLength(0);
  await form.getByRole("button", { name: "Apply catalog filters", exact: true }).click();
  await expect(page.getByText(result.title, { exact: true })).toBeVisible();
  expect(filtered).toHaveLength(1);
  expect(Object.fromEntries(filtered[0].searchParams)).toMatchObject({
    q: "", genre: "Action & Power", theme: "Strategy", demographic: "Seinen", studio: "Synthetic Works",
    mediaType: "TV", status: "RELEASING", releaseYear: "2026", season: "FALL", limit: "100", sort: "NEWEST", minEpisodes: "12", maxEpisodes: "24", minYear: "2020", maxYear: "2030",
  });
  await expect(page.getByText("Unfiltered Starter", { exact: true })).toHaveCount(0);
  await expect(page.getByText("Additional Filter Match 9", { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Show more catalog matches", exact: true }).click();
  await expect(page.getByText("Additional Filter Match 9", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Hide catalog filters (13 applied)", exact: true })).toBeVisible();
  await form.getByRole("button", { name: "Clear catalog filters", exact: true }).click();
  await expect(page.getByText(result.title, { exact: true })).toHaveCount(0);
  await expect(form.getByLabel("Source genre", { exact: true })).toHaveValue("");
});

test("catalog filter replacement fences late results and failed searches retry", async ({ page }) => {
  let release = () => {};
  let blocked = false;
  let failures = 0;
  const held = new Promise<void>(resolve => { release = resolve; });
  await page.route("http://localhost:8080/api/v1**", async route => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace("/api/v1", "");
    const respond = (body: unknown, status = 200) => route.fulfill({ status, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: false });
    if (path === "/tracker") return respond({ items: [] });
    if (path === "/anime") {
      const genre = url.searchParams.get("genre");
      if (genre === "Old") { blocked = true; await held; return respond({ items: [{ ...result, title: "Stale Filter Result" }] }); }
      if (genre === "Fail" && failures++ === 0) return respond({ error: "synthetic_read_failure" }, 500);
      return respond({ items: genre ? [result] : [] });
    }
    return respond({}, 404);
  });
  await page.goto("/");
  await page.getByRole("button", { name: "Catalog filters", exact: true }).click();
  const form = page.getByRole("form", { name: "Catalog metadata filters", exact: true });
  await form.getByLabel("Source genre", { exact: true }).fill("Old");
  await form.getByRole("button", { name: "Apply catalog filters", exact: true }).click();
  await expect.poll(() => blocked).toBe(true);
  await form.getByLabel("Source genre", { exact: true }).fill("New");
  await form.getByRole("button", { name: "Apply catalog filters", exact: true }).click();
  await expect(page.getByText(result.title, { exact: true })).toBeVisible();
  release();
  await expect(page.getByText("Stale Filter Result", { exact: true })).toHaveCount(0);
  await form.getByLabel("Source genre", { exact: true }).fill("Fail");
  await form.getByRole("button", { name: "Apply catalog filters", exact: true }).click();
  await expect(page.getByRole("button", { name: "Retry catalog search", exact: true })).toBeVisible();
  await expect(page.getByText(result.title, { exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "Retry catalog search", exact: true }).click();
  await expect(page.getByText(result.title, { exact: true })).toBeVisible();
  expect(failures).toBe(2);
});
