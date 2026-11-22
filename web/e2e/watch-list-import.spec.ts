import { expect, test } from "@playwright/test";

test("watch-list import uploads only after submit and shows added and skipped counts", async ({ page }) => {
  const data = { version: 1, entries: [{ animeId: "synthetic-title", watchStatus: "WATCHING" }] };
  const uploads: unknown[] = [];
  await page.route("http://localhost:8080/api/v1**", async route => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const headers = {
      "Access-Control-Allow-Origin": "http://127.0.0.1:3000",
      "Access-Control-Allow-Headers": "Authorization, Content-Type",
      "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
    };
    const send = (json: unknown) => route.fulfill({ status: 200, headers, json });
    if (request.method() === "OPTIONS") return route.fulfill({ status: 204, headers });
    if (path === "") return send({ authenticationRequired: true });
    if (path === "/auth/login") return send({
      token: "synthetic-test-token",
      expiresAt: new Date(Date.now() + 60000).toISOString(),
      user: { id: "synthetic-owner", email: "import@example.test", emailVerified: true },
    });
    if (path === "/tracker/import") {
      expect(request.method()).toBe("POST");
      uploads.push(request.postDataJSON());
      return send({ added: 1, skipped: 0 });
    }
    return send({ items: [] });
  });
  await page.goto("/");
  await page.getByLabel("Email", { exact: true }).fill("import@example.test");
  await page.getByLabel("Password", { exact: true }).fill("synthetic-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await page.getByRole("button", { name: "Import watch list", exact: true }).click();
  await page.getByLabel("Anime Intelligence watch-list JSON").setInputFiles({
    name: "watch-list.json",
    mimeType: "application/json",
    buffer: Buffer.from(JSON.stringify(data)),
  });
  expect(uploads).toEqual([]);
  await page.getByRole("button", { name: "Import new titles" }).click();
  await expect(page.getByText("Watch list imported. Added 1. Skipped 0 already in your library.")).toBeVisible();
  expect(uploads).toEqual([data]);
});
