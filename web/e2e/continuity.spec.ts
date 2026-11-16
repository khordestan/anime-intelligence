import { test, expect } from "@playwright/test";

const root = "10000000-0000-4000-8000-000000000091";
const middle = "10000000-0000-4000-8000-000000000093";
const later = "10000000-0000-4000-8000-000000000092";
const headers = {
  "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
  "Access-Control-Allow-Headers": "Authorization, Content-Type",
  "Access-Control-Allow-Methods": "GET, POST, PATCH, PUT, DELETE, OPTIONS",
};
const graph = {
  rootId: root, limit: 50, truncated: true,
  nodes: [
    { id: root, title: "Synthetic Starting Season", mediaType: "TV", status: "FINISHED", episodeCountKnown: true, episodeCount: 0, distance: 0, provider: "synthetic", observedAt: "2026-01-01T00:00:00Z" },
    { id: middle, title: "Synthetic Middle Season", mediaType: "TV", status: "FINISHED", episodeCountKnown: false, distance: 1, provider: "synthetic" },
    { id: later, title: "Synthetic Distant Movie", mediaType: "MOVIE", status: "ANNOUNCED", episodeCountKnown: false, distance: 2, provider: "synthetic" },
  ],
  edges: [{ fromId: root, toId: middle, providers: ["synthetic", "synthetic-other"] }, { fromId: middle, toId: later, providers: ["synthetic"] }],
};
test("linked-title browsing is explicit, bounded, sourced and navigates beyond direct neighbors", async ({ page }) => {
  let graphRequests = 0;
  await page.route("http://localhost:8080/api/v1**", async route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    const respond = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: false });
    if (path === "/tracker") return respond({ items: [] });
    if (path === "/anime") return respond({ items: [{ id: root, title: "Synthetic Starting Season", mediaType: "TV" }] });
    if (path.endsWith("/details")) {
      const id = path.split("/")[2];
      return respond({ anime: { id, title: id === root ? "Synthetic Starting Season" : "Synthetic Distant Movie", episodeCountKnown: false }, genres: [], tags: [], studios: [], synonyms: [], relations: [], episodes: [] });
    }
    if (path === "/anime/" + root + "/continuity") { graphRequests++; return respond(graph); }
    return route.fulfill({ status: 404, headers, body: "{}" });
  });
  await page.goto("/");
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic Starting");
  await page.getByRole("button", { name: "View catalog details Synthetic Starting Season", exact: true }).click();
  const details = page.getByRole("region", { name: "Catalog details Synthetic Starting Season", exact: true });
  await expect(details.getByRole("button", { name: "Browse linked titles", exact: true })).toBeVisible();
  expect(graphRequests).toBe(0);
  await details.getByRole("button", { name: "Browse linked titles", exact: true }).click();
  const linked = page.getByRole("region", { name: "Linked titles for Synthetic Starting Season", exact: true });
  await expect(linked.getByText("Showing a partial view of up to 50 titles. Open a linked title to explore from there.", { exact: true })).toBeVisible();
  await expect(linked.getByText(/0 episodes/)).toBeVisible();
  await expect(linked.getByText(/MOVIE · ANNOUNCED · Episode total unknown · 2 links away/)).toBeVisible();
  await expect(linked.getByText(/Sources: synthetic, synthetic-other/)).toBeVisible();
  await expect(linked.getByText(/not a recommended watch order/)).toBeVisible();
  await linked.getByRole("button", { name: "View linked title: Synthetic Distant Movie", exact: true }).click();
  await expect(details.getByRole("heading", { name: "Synthetic Distant Movie", exact: true })).toBeVisible();
  await expect(linked).toHaveCount(0);
});

test("linked-title errors retry and closed panels discard late results", async ({ page }) => {
  let attempts = 0;
  let release: (() => void) | undefined;
  const delayed = new Promise<void>(resolve => { release = resolve; });
  await page.route("http://localhost:8080/api/v1**", async route => {
    const path = new URL(route.request().url()).pathname.replace("/api/v1", "");
    const respond = (body: unknown) => route.fulfill({ status: 200, headers, contentType: "application/json", body: JSON.stringify(body) });
    if (route.request().method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return respond({ authenticationRequired: false });
    if (path === "/tracker") return respond({ items: [] });
    if (path === "/anime") return respond({ items: [{ id: root, title: "Synthetic Starting Season", mediaType: "TV" }] });
    if (path.endsWith("/details")) return respond({ anime: { id: root, title: "Synthetic Starting Season", episodeCountKnown: false }, genres: [], tags: [], studios: [], synonyms: [], relations: [], episodes: [] });
    if (path.endsWith("/continuity")) {
      attempts++;
      if (attempts === 1) return route.fulfill({ status: 500, headers, body: "{}" });
      if (attempts === 2) await delayed;
      return respond({ ...graph, truncated: false, nodes: [graph.nodes[0]], edges: [] });
    }
    return route.fulfill({ status: 404, headers, body: "{}" });
  });
  await page.goto("/");
  await page.getByPlaceholder("Search starter catalog…").fill("Synthetic Starting");
  await page.getByRole("button", { name: "View catalog details Synthetic Starting Season", exact: true }).click();
  await page.getByRole("button", { name: "Browse linked titles", exact: true }).click();
  const linked = page.getByRole("region", { name: "Linked titles for Synthetic Starting Season", exact: true });
  await expect(linked.getByText(/Could not load linked titles/)).toBeVisible();
  await linked.getByRole("button", { name: "Retry linked titles", exact: true }).click();
  await expect.poll(() => attempts).toBe(2);
  await linked.getByRole("button", { name: "Close linked titles", exact: true }).click();
  release?.();
  await expect(linked).toHaveCount(0);
  await page.getByRole("button", { name: "Browse linked titles", exact: true }).click();
  await expect(linked.getByText("No linked prequels or sequels are available.", { exact: true })).toBeVisible();
  expect(attempts).toBe(3);
});
